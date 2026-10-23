# Step execution fixture

Only edit tests/calc/steps/. Register the missing doubling step using State.Value. Existing number and result definitions show the parameterized convention. The declared replay command injects an incorrect number definition on its first run, to test repair from runtime feedback. Repair the step implementation when replay reports expected and actual values. Never edit replay.sh, the scenario shim, the Registry or the architecture.
