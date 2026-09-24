# One line per tool call, in order: "Skill <name>" for a skill, otherwise the
# tool's name. reviewed.sh reads the same list.
.. | objects | select(.type? == "tool_use")
| if .name == "Skill" then "Skill " + (.input.skill // "") else .name end
