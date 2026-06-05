package core_references

type ReferencesTest struct {
	Name          string
	SQL           string
	DefaultSchema string
	SkipDialects  []string
	ExpectedRefs  []RelationRef
	ExpectedVtabs []RelationRef
}

func GetTestMetadata(schema string) Metadata {
	return Metadata{
		DefaultDB:     "db",
		DefaultSchema: schema,
		CurrentSchema: "", // Empty means it will fall back to DefaultSchema
		Schemas: []Schema{
			{Name: schema, Tables: []Table{
				{Name: "t1", Columns: []Column{{Name: "c1", Type: "int", Nullable: false}}},
				{Name: "t2", Columns: []Column{{Name: "c1", Type: "int", Nullable: false}, {Name: "c2", Type: "int", Nullable: false}}},
				{Name: "t3", Columns: []Column{{Name: "c31", Type: "int", Nullable: false}, {Name: "c32", Type: "int", Nullable: false}}},
			}},
		},
	}
}

func GetReferencesSelectTests(schema string) []ReferencesTest {
	return []ReferencesTest{
		{
			Name:          "simple select from table",
			SQL:           "SELECT * FROM t1",
			DefaultSchema: schema,
			ExpectedRefs:  []RelationRef{{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0}},
			ExpectedVtabs: []RelationRef{},
		},
		{
			Name:          "select with table alias",
			SQL:           "SELECT * FROM t1 tt1",
			DefaultSchema: schema,
			ExpectedRefs:  []RelationRef{{Schema: schema, Table: "t1", Alias: "tt1", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0}},
			ExpectedVtabs: []RelationRef{},
		},
		{
			Name:          "select with schema.table",
			SQL:           "SELECT * FROM " + schema + ".t1",
			DefaultSchema: schema,
			ExpectedRefs:  []RelationRef{{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0}},
			ExpectedVtabs: []RelationRef{},
		},
		{
			Name:          "join with multiple tables",
			SQL:           "SELECT * FROM t1 tt1 JOIN t2 tt2 ON tt1.c1 = tt2.c1",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", Alias: "tt1", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0},
				{Schema: schema, Table: "t2", Alias: "tt2", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0},
			},
			ExpectedVtabs: []RelationRef{},
		},
		{
			Name:          "subquery in from clause",
			SQL:           "SELECT * FROM (SELECT c1 FROM t1) subq",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: "", Table: "subq", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0},
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 1},
			},
			ExpectedVtabs: []RelationRef{
				{Table: "subq", Columns: []Column{{Name: "c1", Type: "int", Nullable: false}}},
			},
		},
		{
			Name:          "subquery in from clause with alias",
			SQL:           "SELECT * FROM (SELECT c1 as x1, c2 x2 FROM t2) subq",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: "", Table: "subq", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0},
				{Schema: schema, Table: "t2", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 1},
			},
			ExpectedVtabs: []RelationRef{
				{Table: "subq", Columns: []Column{{Name: "x1", Type: "int", Nullable: false}, {Name: "x2", Type: "int", Nullable: false}}},
			},
		},
		{
			Name:          "subquery with explicit column list",
			SQL:           "SELECT * FROM (SELECT c1, c2 FROM t2) subq(col1, col2)",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: "", Table: "subq", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0},
				{Schema: schema, Table: "t2", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 1},
			},
			ExpectedVtabs: []RelationRef{
				{Table: "subq", Columns: []Column{{Name: "col1", Type: "int", Nullable: false}, {Name: "col2", Type: "int", Nullable: false}}},
			},
		},
		{
			Name:          "CTE",
			SQL:           "WITH cte AS (SELECT c1 FROM t1) SELECT * FROM cte",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 8, ScopeEndPos: 14, NestingLevel: 1},
				{Schema: "", Table: "cte", Alias: "", ScopeStartPos: 16, ScopeEndPos: -1, NestingLevel: 0},
			},
			ExpectedVtabs: []RelationRef{
				{Table: "cte", Columns: []Column{{Name: "c1", Type: "int", Nullable: false}}, ScopeStartPos: 15, ScopeEndPos: -1, NestingLevel: 0},
			},
		},
		{
			Name:          "CTE select list qualified column with table alias",
			SQL:           "WITH cte AS (SELECT t.c1 FROM t1 t) SELECT * FROM cte",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", Alias: "t", ScopeStartPos: 8, ScopeEndPos: 18, NestingLevel: 1},
				{Schema: "", Table: "cte", Alias: "", ScopeStartPos: 20, ScopeEndPos: -1, NestingLevel: 0},
			},
			ExpectedVtabs: []RelationRef{
				{Table: "cte", Columns: []Column{{Name: "c1", Type: "int", Nullable: false}}, ScopeStartPos: 19, ScopeEndPos: -1, NestingLevel: 0},
			},
		},
		// CTE expression projection names: PostgreSQL InferColumnsFromSubquery; SQLite token heuristic may differ (sqlite tests do not assert column names).
		{
			Name:          "CTE select list expression AS collabel",
			SQL:           "WITH cte AS (SELECT length('x') AS num FROM t1) SELECT * FROM cte",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 8, ScopeEndPos: 21, NestingLevel: 1},
				{Schema: "", Table: "cte", Alias: "", ScopeStartPos: 23, ScopeEndPos: -1, NestingLevel: 0},
			},
			ExpectedVtabs: []RelationRef{
				{Table: "cte", Columns: []Column{{Name: "num", Type: "unknown", Nullable: true}}, ScopeStartPos: 22, ScopeEndPos: -1, NestingLevel: 0},
			},
		},
		{
			Name:          "CTE select list expression implicit alias",
			SQL:           "WITH cte AS (SELECT length('x') len FROM t1) SELECT * FROM cte",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 8, ScopeEndPos: 19, NestingLevel: 1},
				{Schema: "", Table: "cte", Alias: "", ScopeStartPos: 21, ScopeEndPos: -1, NestingLevel: 0},
			},
			ExpectedVtabs: []RelationRef{
				{Table: "cte", Columns: []Column{{Name: "len", Type: "unknown", Nullable: true}}, ScopeStartPos: 20, ScopeEndPos: -1, NestingLevel: 0},
			},
		},
		{
			Name:          "CTE select list bare expression label",
			SQL:           "WITH cte AS (SELECT length('x') FROM t1) SELECT * FROM cte",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 8, ScopeEndPos: 17, NestingLevel: 1},
				{Schema: "", Table: "cte", Alias: "", ScopeStartPos: 19, ScopeEndPos: -1, NestingLevel: 0},
			},
			ExpectedVtabs: []RelationRef{
				{Table: "cte", Columns: []Column{{Name: "length('x')", Type: "unknown", Nullable: true}}, ScopeStartPos: 18, ScopeEndPos: -1, NestingLevel: 0},
			},
		},
		{
			Name:          "CTE with subquery",
			SQL:           "WITH cte AS (SELECT * FROM (SELECT c1 FROM t1) subq) SELECT * FROM cte",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: "", Table: "subq", Alias: "", ScopeStartPos: 8, ScopeEndPos: 24, NestingLevel: 1},
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 8, ScopeEndPos: 24, NestingLevel: 1},
				{Schema: "", Table: "cte", Alias: "", ScopeStartPos: 26, ScopeEndPos: -1, NestingLevel: 0},
			},
			ExpectedVtabs: []RelationRef{
				{Table: "subq", Columns: []Column{{Name: "c1", Type: "int", Nullable: false}}, ScopeStartPos: 7, ScopeEndPos: 24, NestingLevel: 0},
				{Table: "cte", Columns: []Column{{Name: "c1", Type: "int", Nullable: false}}, ScopeStartPos: 25, ScopeEndPos: -1, NestingLevel: 0},
			},
		},
		{
			Name:          "recursive CTE",
			SQL:           "WITH RECURSIVE cte AS (SELECT c1 FROM t1 UNION ALL SELECT c1 FROM cte WHERE c1 < 10) SELECT * FROM cte",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 10, ScopeEndPos: 36, NestingLevel: 1},
				{Schema: "", Table: "cte", Alias: "", ScopeStartPos: 38, ScopeEndPos: -1, NestingLevel: 0},
			},
			ExpectedVtabs: []RelationRef{
				{Table: "cte", Columns: []Column{{Name: "c1", Type: "int", Nullable: false}}, ScopeStartPos: 37, ScopeEndPos: -1, NestingLevel: 0},
			},
		},
		{
			Name:          "multiple CTEs",
			SQL:           "WITH cte1 AS (SELECT c1 FROM t1), cte2 AS (SELECT c2 FROM t2) SELECT * FROM cte1 JOIN cte2 ON cte1.c1 = cte2.c2",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 8, ScopeEndPos: 14, NestingLevel: 1},
				{Schema: schema, Table: "t2", Alias: "", ScopeStartPos: 23, ScopeEndPos: 29, NestingLevel: 1},
				{Schema: "", Table: "cte1", Alias: "", ScopeStartPos: 31, ScopeEndPos: -1, NestingLevel: 0},
				{Schema: "", Table: "cte2", Alias: "", ScopeStartPos: 31, ScopeEndPos: -1, NestingLevel: 0},
			},
			ExpectedVtabs: []RelationRef{
				{Table: "cte1", Columns: []Column{{Name: "c1", Type: "int", Nullable: false}}, ScopeStartPos: 15, ScopeEndPos: -1, NestingLevel: 0},
				{Table: "cte2", Columns: []Column{{Name: "c2", Type: "int", Nullable: false}}, ScopeStartPos: 30, ScopeEndPos: -1, NestingLevel: 0},
			},
		},
		{
			Name:          "nested subqueries with aliases",
			SQL:           "SELECT * FROM (SELECT * FROM (SELECT c1 FROM t1) inner_subq) outer_subq",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: "", Table: "outer_subq", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0},
				{Schema: "", Table: "inner_subq", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 1},
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 2},
			},
			ExpectedVtabs: []RelationRef{
				{Table: "outer_subq", Columns: []Column{{Name: "c1", Type: "int", Nullable: false}}, ScopeStartPos: 0, ScopeEndPos: 0, NestingLevel: 0},
				{Table: "inner_subq", Columns: []Column{{Name: "c1", Type: "int", Nullable: false}}, ScopeStartPos: 0, ScopeEndPos: 0, NestingLevel: 1},
			},
		},
		{
			Name:          "quoted identifiers",
			SQL:           `SELECT * FROM "t1" "tt1"`,
			DefaultSchema: schema,
			ExpectedRefs:  []RelationRef{{Schema: schema, Table: "t1", Alias: "tt1", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0}},
			ExpectedVtabs: []RelationRef{},
		},
		{
			Name:          "left join",
			SQL:           "SELECT * FROM t1 LEFT JOIN t2 ON t1.c1 = t2.c1",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0},
				{Schema: schema, Table: "t2", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0},
			},
			ExpectedVtabs: []RelationRef{},
		},
		{
			Name:          "cross join",
			SQL:           "SELECT * FROM t1 CROSS JOIN t2",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0},
				{Schema: schema, Table: "t2", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0},
			},
			ExpectedVtabs: []RelationRef{},
		},
		{
			Name:          "materialized view",
			SQL:           "SELECT * FROM v1",
			DefaultSchema: schema,
			ExpectedRefs:  []RelationRef{{Schema: schema, Table: "v1", Alias: "", ScopeStartPos: 0, ScopeEndPos: -1, NestingLevel: 0}},
			ExpectedVtabs: []RelationRef{},
		},
		{
			Name:          "empty query",
			SQL:           "",
			DefaultSchema: schema,
			ExpectedRefs:  []RelationRef{},
			ExpectedVtabs: []RelationRef{},
		},
		{
			Name:          "multiple FROM clauses in same query",
			SQL:           "SELECT * FROM t1; SELECT * FROM t1",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 0, ScopeEndPos: 7, NestingLevel: 0},
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 9, ScopeEndPos: -1, NestingLevel: 0},
			},
			ExpectedVtabs: []RelationRef{},
		},
		{
			Name:          "multiple statements - different tables",
			SQL:           "SELECT * FROM t1; SELECT * FROM t2",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 0, ScopeEndPos: 7, NestingLevel: 0},
				{Schema: schema, Table: "t2", Alias: "", ScopeStartPos: 9, ScopeEndPos: -1, NestingLevel: 0},
			},
			ExpectedVtabs: []RelationRef{},
		},
		{
			Name:          "multiple statements - CTE not available in first statement",
			SQL:           "SELECT * FROM t1; WITH cte AS (SELECT * FROM t2) SELECT * FROM cte",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 0, ScopeEndPos: 7, NestingLevel: 0},
				{Schema: schema, Table: "t2", Alias: "", ScopeStartPos: 17, ScopeEndPos: 23, NestingLevel: 1},
				{Schema: "", Table: "cte", Alias: "", ScopeStartPos: 25, ScopeEndPos: -1, NestingLevel: 0},
			},
			ExpectedVtabs: []RelationRef{
				{
					Table: "cte",
					Columns: []Column{
						{Name: "c1", Type: "int", Nullable: false},
						{Name: "c2", Type: "int", Nullable: false},
					},
					ScopeStartPos: 24,
					ScopeEndPos:   -1,
					NestingLevel:  0,
				},
			},
		},
		{
			Name:          "multiple statements - invalid first, valid second",
			SQL:           "SELECT FROM azerty; SELECT * FROM t1",
			DefaultSchema: schema,
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "azerty", Alias: "", ScopeStartPos: 0, ScopeEndPos: 5, NestingLevel: 0},
				{Schema: schema, Table: "t1", Alias: "", ScopeStartPos: 7, ScopeEndPos: -1, NestingLevel: 0},
			},
			ExpectedVtabs: []RelationRef{},
		},
	}
}

func GetReferencesUpdateTests(schema string) []ReferencesTest {
	return []ReferencesTest{
		{
			Name:         "simple update",
			SQL:          "UPDATE t2 SET t2.c2 = 0",
			ExpectedRefs: []RelationRef{{Schema: schema, Table: "t2"}},
		},
		{
			Name:         "schema qualified update",
			SQL:          "UPDATE " + schema + ".t2 SET t2.c2 = 1",
			ExpectedRefs: []RelationRef{{Schema: schema, Table: "t2"}},
		},
		{
			Name:         "update with from (PG-specific)",
			SQL:          "UPDATE t2 SET t2.c2 = t1.c1 FROM t1",
			SkipDialects: []string{"sqlite", "mysql"},
			ExpectedRefs: []RelationRef{{Schema: schema, Table: "t2"}, {Schema: schema, Table: "t1"}},
		},
		{
			Name:         "update with alias (AS)",
			SQL:          "UPDATE t2 AS tt2 SET tt2.c2 = 0",
			ExpectedRefs: []RelationRef{{Schema: schema, Table: "t2", Alias: "tt2"}},
		},
		{
			Name:         "update with alias (implicit)",
			SQL:          "UPDATE t2 tt2 SET tt2.c2 = 0",
			ExpectedRefs: []RelationRef{{Schema: schema, Table: "t2", Alias: "tt2"}},
		},
		{
			Name:         "update with cte and from",
			SQL:          "WITH cte AS (SELECT * FROM t1) UPDATE t2 SET t2.c2 = cte.c1 FROM cte",
			SkipDialects: []string{"sqlite", "mysql"},
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", NestingLevel: 1},
				{Schema: schema, Table: "t2"},
				{Schema: "", Table: "cte"},
			},
			ExpectedVtabs: []RelationRef{
				{Table: "cte", Columns: []Column{{Name: "c1", Type: "int", Nullable: false}}},
			},
		},
		{
			Name:         "update with multiple ctes",
			SQL:          "WITH cte1 AS (SELECT * FROM t1), cte2 AS (SELECT * FROM t2) UPDATE t2 SET t2.c2 = cte1.c1 FROM cte1",
			SkipDialects: []string{"sqlite", "mysql"},
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", NestingLevel: 1},
				{Schema: schema, Table: "t2", NestingLevel: 1},
				{Schema: schema, Table: "t2"},
				{Schema: "", Table: "cte1"},
			},
			ExpectedVtabs: []RelationRef{
				{Table: "cte1", Columns: []Column{{Name: "c1", Type: "int", Nullable: false}}},
				{Table: "cte2", Columns: []Column{{Name: "c1", Type: "int", Nullable: false}, {Name: "c2", Type: "int", Nullable: false}}},
			},
		},
		{
			Name:         "update with cte alias in from",
			SQL:          "WITH cte AS (SELECT * FROM t1) UPDATE t2 SET t2.c2 = tt1.c1 FROM cte tt1",
			SkipDialects: []string{"sqlite", "mysql"},
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t1", NestingLevel: 1},
				{Schema: schema, Table: "t2"},
				{Schema: "", Table: "cte", Alias: "tt1"},
			},
			ExpectedVtabs: []RelationRef{
				{Table: "cte", Columns: []Column{{Name: "c1", Type: "int", Nullable: false}}},
			},
		},
		{
			Name:         "update with alias and from alias",
			SQL:          "UPDATE t2 tt2 SET tt2.c2 = tt1.c1 FROM t1 tt1",
			SkipDialects: []string{"sqlite", "mysql"},
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t2", Alias: "tt2"},
				{Schema: schema, Table: "t1", Alias: "tt1"},
			},
		},
		{
			Name:         "update with from join chain",
			SQL:          "UPDATE t2 SET t2.c2 = tt3.c31 FROM t1 tt1 JOIN t3 tt3 ON tt1.c1 = tt3.c31",
			SkipDialects: []string{"sqlite", "mysql"},
			ExpectedRefs: []RelationRef{
				{Schema: schema, Table: "t2"},
				{Schema: schema, Table: "t1", Alias: "tt1"},
				{Schema: schema, Table: "t3", Alias: "tt3"},
			},
		},
	}
}
