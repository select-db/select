# One line per tool call, in order: "Skill <name>" without a plugin prefix,
# otherwise the tool's name. reviewed.sh reads the same list.
.. | objects | select(.type? == "tool_use")
| if .name == "Skill" then "Skill " + ((.input.skill // "") | sub(".*:"; "")) else .name end
