package remote

import "time"

type SessionStatus string

const (
	StatusPending SessionStatus = "pending"
	StatusOffered SessionStatus = "offered"
	StatusActive  SessionStatus = "active"
	StatusClosed  SessionStatus = "closed"
)

type Session struct {
	ID                 string        `json:"id"`
	Target             string        `json:"target"`
	Viewer             string        `json:"viewer"`
	Status             SessionStatus `json:"status"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
	OfferSDP           string        `json:"offer_sdp,omitempty"`
	AnswerSDP          string        `json:"answer_sdp,omitempty"`
	ViewerICECandidates []string     `json:"viewer_ice_candidates,omitempty"`
	HostICECandidates   []string     `json:"host_ice_candidates,omitempty"`
	RoutedHostID       string        `json:"routed_host_id,omitempty"`
	RoutedHostPublicID string        `json:"routed_host_public_id,omitempty"`
	RoutedHostname     string        `json:"routed_hostname,omitempty"`
	DispatchState      string        `json:"dispatch_state,omitempty"`
	DispatchCount      int           `json:"dispatch_count,omitempty"`
	LastDispatchAt     *time.Time    `json:"last_dispatch_at,omitempty"`
	LastHostAckAt      *time.Time    `json:"last_host_ack_at,omitempty"`
}

type CreateSessionRequest struct {
	Target string `json:"target"`
	Viewer string `json:"viewer"`
}

type SDPRequest struct {
	SDP string `json:"sdp"`
}

type CandidateRequest struct {
	Candidate string `json:"candidate"`
	Source    string `json:"source,omitempty"`
}
