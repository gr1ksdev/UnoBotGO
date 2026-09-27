package ranking

const (
	// PlacementPolicyV1 counts all and only valid final placements. Late join and
	// previous leaves/reentries do not affect eligibility. N<2 grants no awards.
	PlacementPolicyV1         = "completed-placements-v1"
	StatusNeedsDecision       = "needs_product_decision"
	StatusScored              = "scored"
	StatusInsufficientPlayers = "insufficient_eligible_players"
)

// Eligible describes a concluded player's placement, including the remaining
// player recorded by the engine. A historical participant with no placement
// stays in the audit result but is excluded from N and ranking statistics.
func (p Player) Eligible() bool {
	return p.Position > 0 && ((p.FinalStatus == "went_out" && p.WentOut) || (p.FinalStatus == "playing" && !p.WentOut))
}

func (r Result) EligibleCount() int {
	n := 0
	for _, p := range r.Players {
		if p.Eligible() {
			n++
		}
	}
	return n
}

// ScoringStatus is used only after Validate. A selected policy with N<2 records
// an explicit no-award result; it must not update any accumulated statistics.
func (r Result) ScoringStatus() string {
	if r.PolicyVersion == "" {
		return StatusNeedsDecision
	}
	if r.EligibleCount() < 2 {
		return StatusInsufficientPlayers
	}
	return StatusScored
}

// Prepare evaluates a final result on a copy, retaining every audit participant
// and the engine's positions. It performs no I/O and never mutates live gameplay.
// Retrying an already prepared result validates its original payload, not a
// recalculation which could conceal divergent scores.
func Prepare(result Result) (Result, error) {
	if err := result.Validate(); err != nil {
		return Result{}, err
	}
	r := result.Clone()
	if r.PolicyVersion != "" {
		return r, nil
	}
	r.PolicyVersion = PlacementPolicyV1
	n := r.EligibleCount()
	if n >= 2 {
		for i := range r.Players {
			p := &r.Players[i]
			if !p.Eligible() {
				continue
			}
			score, err := Score(r.RankingSystem, n, p.Position)
			if err != nil {
				return Result{}, err
			}
			p.Score = score
		}
	}
	return r, r.Validate()
}
