# One line per tool call, in order: "Skill <name>" for a skill, otherwise the
# tool's name, for pr-review-gate.sh fanned-out.
.. | objects | select(.type? == "tool_use")
| if .name == "Skill" then "Skill " + (.input.skill // "") else .name end
