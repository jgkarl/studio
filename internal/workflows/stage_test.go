package workflows

import "testing"

// Every stage listed in a transition target must also be a real, known stage — a typo'd stage
// name here would silently make a transition impossible to reach in the UI (the <select> in the
// advance-stage form is built from this map) without ever failing loudly.
func TestStageTransitionsReferenceKnownStages(t *testing.T) {
	known := map[Stage]bool{}
	for _, s := range Stages {
		known[s] = true
	}
	for from, tos := range StageTransitions {
		if !known[from] {
			t.Errorf("StageTransitions has an entry for unknown stage %q", from)
		}
		for _, to := range tos {
			if !known[to] {
				t.Errorf("StageTransitions[%q] references unknown stage %q", from, to)
			}
		}
	}
}

// Every stage must have a display label, or the stage badge/trail renders blank.
func TestEveryStageHasALabel(t *testing.T) {
	for _, s := range Stages {
		if StageLabels[s] == "" {
			t.Errorf("stage %q has no entry in StageLabels", s)
		}
	}
}

func TestStageTransitionsAllowedMoves(t *testing.T) {
	cases := []struct {
		from    Stage
		to      Stage
		allowed bool
	}{
		{StageIngest, StageDescribe, true},
		{StageIngest, StageTreatment, false}, // can't skip ahead
		{StageDescribe, StageFixateCurrentState, true},
		{StageFixateCurrentState, StageTreatment, true},
		{StageFixateCurrentState, StageIngest, false}, // no going backward
		{StageTreatment, StageTreatment, true},        // self-loop: treatment can repeat
		{StageTreatment, StageWaiting, true},
		{StageTreatment, StageFixateNewState, true},
		{StageWaiting, StageTreatment, true},
		{StageWaiting, StageWaiting, true}, // self-loop: waiting can repeat
		{StageWaiting, StageFixateNewState, true},
		{StageFixateNewState, StageHandoverDone, true},
		{StageFixateNewState, StageWaiting, false}, // no going back into treatment/waiting
		{StageHandoverDone, StageIngest, false},    // terminal — no moves out
	}
	for _, c := range cases {
		allowed := false
		for _, to := range StageTransitions[c.from] {
			if to == c.to {
				allowed = true
				break
			}
		}
		if allowed != c.allowed {
			t.Errorf("StageTransitions[%q] allows -> %q = %v, want %v", c.from, c.to, allowed, c.allowed)
		}
	}
}
