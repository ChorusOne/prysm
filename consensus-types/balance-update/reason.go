package balanceupdate

type Reason = int

const (
	Test Reason = 0

	AttesterAttestation       = iota
	AttesterAttestationSource = iota
	AttesterAttestationTarget = iota
	AttesterHead              = iota
	AttesterInactivity        = iota
	ProposerAttestations      = iota
	ProposerProposal          = iota
	ProposerWhistleblowing    = iota
	ValidatorDeposit          = iota
	ValidatorSlashing         = iota
	ValidatorWhistleblowing   = iota
	ValidatorWithdrawal       = iota

	TotalReasonsCount = iota
)

func ReasonID(r Reason) string {
	switch r {
	case AttesterAttestation:
		return "attesterAttestation"
	case AttesterAttestationSource:
		return "attesterAttestationSource"
	case AttesterAttestationTarget:
		return "attesterAttestationTarget"
	case AttesterHead:
		return "attesterHead"
	case AttesterInactivity:
		return "attesterInactivity"
	case ProposerAttestations:
		return "proposerAttestations"
	case ProposerProposal:
		return "proposerProposal"
	case ProposerWhistleblowing:
		return "proposerWhistleblowing"
	case ValidatorDeposit:
		return "validatorDeposit"
	case ValidatorSlashing:
		return "validatorSlashing"
	case ValidatorWhistleblowing:
		return "validatorWhistleblowing"
	case ValidatorWithdrawal:
		return "validatorWithdrawal"
	default:
		return "unknown"
	}
}
