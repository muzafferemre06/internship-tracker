package domain

// OpportunityLifecycle describes the durable user-visible lifecycle of a
// canonical opportunity. It is intentionally separate from application
// tracking and from the internal active/merged canonicalization state.
type OpportunityLifecycle string

const (
	OpportunityNew      OpportunityLifecycle = "yeni"
	OpportunityOpen     OpportunityLifecycle = "acik"
	OpportunityReviewed OpportunityLifecycle = "incelendi"
	OpportunityApplied  OpportunityLifecycle = "basvuruldu"
	OpportunityExpired  OpportunityLifecycle = "suresi_doldu"
	OpportunityClosed   OpportunityLifecycle = "kapatildi"
	OpportunityArchived OpportunityLifecycle = "arsivlendi"
)

func (l OpportunityLifecycle) Valid() bool {
	switch l {
	case OpportunityNew, OpportunityOpen, OpportunityReviewed, OpportunityApplied,
		OpportunityExpired, OpportunityClosed, OpportunityArchived:
		return true
	default:
		return false
	}
}
