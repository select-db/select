package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"selectDb/internal/graph"
)

func buildFileURI(workspaceID, path string) string {
	return "selectdb://workspaces/" + workspaceID + "/" + path
}

func (s *Search) Search(params SearchParams) (*SearchResult, error) {
	workspacePath, err := s.workspaceRootPath(params.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace: %w", err)
	}

	rgPath, err := getRipgrepBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to get ripgrep binary: %w", err)
	}

	args := []string{"--json", "--no-heading", "--line-number", "--column", "--with-filename"}

	if !params.UseRegex {
		args = append(args, "--fixed-strings")
	}
	if params.CaseSensitive {
		args = append(args, "--case-sensitive")
	} else {
		args = append(args, "--ignore-case")
	}
	if params.WholeWord {
		args = append(args, "--word-regexp")
	}
	if params.IncludePattern != "" {
		args = append(args, "--glob", params.IncludePattern)
	}
	if params.ExcludePattern != "" {
		args = append(args, "--glob", "!"+params.ExcludePattern)
	}
	args = append(args, "--", params.Pattern)

	cmd := exec.CommandContext(s.ctx, rgPath, args...)
	cmd.Dir = workspacePath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err = cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return &SearchResult{}, nil
		}
		return nil, fmt.Errorf("ripgrep error: %w\nstderr: %s", err, stderr.String())
	}

	return parseRipgrepOutput(stdout.String())
}

type rgJSONMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type rgMatch struct {
	Path       struct{ Text string } `json:"path"`
	Lines      struct{ Text string } `json:"lines"`
	LineNumber int                   `json:"line_number"`
	Submatches []struct {
		Match struct{ Text string } `json:"match"`
		Start int                   `json:"start"`
		End   int                   `json:"end"`
	} `json:"submatches"`
}

func parseRipgrepOutput(output string) (*SearchResult, error) {
	fileMap := make(map[string]*SearchFileResult)
	var fileOrder []string
	totalMatches := 0

	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}

		var msg rgJSONMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil || msg.Type != "match" {
			continue
		}

		var match rgMatch
		if err := json.Unmarshal(msg.Data, &match); err != nil {
			continue
		}

		filePath := match.Path.Text
		if _, exists := fileMap[filePath]; !exists {
			fileMap[filePath] = &SearchFileResult{Path: filePath}
			fileOrder = append(fileOrder, filePath)
		}

		for _, submatch := range match.Submatches {
			fileMap[filePath].Matches = append(fileMap[filePath].Matches, SearchMatch{
				Line:       match.LineNumber,
				Column:     submatch.Start + 1,
				LineText:   match.Lines.Text,
				MatchStart: submatch.Start,
				MatchEnd:   submatch.End,
			})
			totalMatches++
		}
	}

	files := make([]SearchFileResult, 0, len(fileOrder))
	for _, path := range fileOrder {
		files = append(files, *fileMap[path])
	}

	return &SearchResult{
		Files:        files,
		TotalFiles:   len(files),
		TotalMatches: totalMatches,
	}, nil
}

func (s *Search) SearchWithNodes(params SearchParams) (*SearchResultWithNodes, error) {
	result, err := s.Search(params)
	if err != nil {
		return nil, err
	}

	// Build a map of file URIs to actual file nodes for O(1) lookup
	fileNodesByURI := make(map[string]*graph.FileNode)
	if workspaceGraph, _ := s.Graph.GetWorkspaceGraph(); workspaceGraph != nil {
		fileURIs := make([]string, len(result.Files))
		for i, r := range result.Files {
			fileURIs[i] = buildFileURI(params.WorkspaceID, r.Path)
		}
		for _, node := range graph.FindNodesByIds(workspaceGraph, fileURIs) {
			if f, ok := node.(*graph.FileNode); ok {
				fileNodesByURI[f.URI] = f
			}
		}
	}

	// Map results to graph nodes
	fileFolders := make([]*graph.FolderNode, 0, len(result.Files))
	for _, r := range result.Files {
		fileNode := fileNodesByURI[buildFileURI(params.WorkspaceID, r.Path)]
		fileFolders = append(fileFolders, mapSearchFileResultToFolderNode(params.WorkspaceID, r, fileNode))
	}

	return &SearchResultWithNodes{
		ResultFolder: &graph.FolderNode{
			ID:          "search::results",
			URI:         "search::results",
			Type:        "folder",
			Name:        strconv.Itoa(result.TotalMatches) + " results in " + strconv.Itoa(result.TotalFiles) + " files",
			Folders:     fileFolders,
			Files:       []*graph.FileNode{},
			DBInstances: []*graph.DBInstanceNode{},
			Badges:      []string{},
		},
		TotalFiles:   result.TotalFiles,
		TotalMatches: result.TotalMatches,
	}, nil
}

func mapSearchFileResultToFolderNode(workspaceID string, r SearchFileResult, fileNode *graph.FileNode) *graph.FolderNode {
	matchNodes := make([]*graph.FileNode, len(r.Matches))
	for i, match := range r.Matches {
		matchNodes[i] = mapMatchToFileNode(workspaceID, r.Path, match, i, fileNode)
	}

	return &graph.FolderNode{
		ID:     "search::file::" + r.Path,
		URI:    buildFileURI(workspaceID, r.Path),
		Type:   "folder",
		Name:   filepath.Base(r.Path),
		Files:  matchNodes,
		Badges: []string{strconv.Itoa(len(r.Matches))},
	}
}

func mapMatchToFileNode(workspaceID, filePath string, match SearchMatch, matchIndex int, fileNode *graph.FileNode) *graph.FileNode {
	preview := strings.TrimSpace(match.LineText)
	if len(preview) > 100 {
		preview = preview[:100] + "..."
	}

	var databases []graph.DatabaseRef
	if fileNode != nil && len(fileNode.Databases) > 0 {
		databases = make([]graph.DatabaseRef, len(fileNode.Databases))
		copy(databases, fileNode.Databases)
	}

	return &graph.FileNode{
		ID:        "search::" + filePath + "::" + strconv.Itoa(matchIndex),
		URI:       buildFileURI(workspaceID, filePath) + "::L" + strconv.Itoa(match.Line) + "::C" + strconv.Itoa(match.Column),
		Type:      "file",
		Name:      strconv.Itoa(match.Line) + ": " + preview,
		Databases: databases,
	}
}
