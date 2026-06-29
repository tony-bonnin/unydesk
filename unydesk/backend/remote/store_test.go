package remote

import (
	"reflect"
	"testing"
	"time"
)

func TestListHostsWithTimeoutUsesStableFleetOrder(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now().UTC()
	firstRegistered := now.Add(-3 * time.Minute)
	secondRegistered := now.Add(-2 * time.Minute)

	store.mu.Lock()
	store.hosts["third"] = Host{
		ID:            "third",
		AccessEnabled: true,
		Hostname:      "beta",
		PublicID:      "300 000 000",
		RegisteredAt:  secondRegistered,
		LastSeenAt:    now,
	}
	store.hosts["first"] = Host{
		ID:            "first",
		AccessEnabled: true,
		Hostname:      "zeta",
		PublicID:      "100 000 000",
		RegisteredAt:  firstRegistered,
		LastSeenAt:    now,
	}
	store.hosts["second"] = Host{
		ID:            "second",
		AccessEnabled: true,
		Hostname:      "alpha",
		PublicID:      "200 000 000",
		RegisteredAt:  secondRegistered,
		LastSeenAt:    now,
	}
	store.mu.Unlock()

	assertHostIDs(t, store.ListHostsWithTimeout(time.Minute), []string{"first", "second", "third"})

	if _, err := store.TouchHost("third"); err != nil {
		t.Fatalf("touch host: %v", err)
	}
	assertHostIDs(t, store.ListHostsWithTimeout(time.Minute), []string{"first", "second", "third"})
}

func TestCreateRoutedReusesMatchingControlSession(t *testing.T) {
	store := NewMemoryStore()
	host := &Host{ID: "host-1", PublicID: "100 000 001", Hostname: "demo-host"}
	req := CreateSessionRequest{Target: host.PublicID, Viewer: "viewer-1"}

	first := store.CreateRouted(req, host)
	second := store.CreateRouted(req, host)

	if first.ID != second.ID {
		t.Fatalf("CreateRouted created %q then %q, want reuse", first.ID, second.ID)
	}
	if first.RoutedHostID != host.ID || second.RoutedHostID != host.ID {
		t.Fatalf("routed host IDs = %q/%q, want %q", first.RoutedHostID, second.RoutedHostID, host.ID)
	}
	if first.DispatchState != "queued" {
		t.Fatalf("first dispatch state = %q, want queued", first.DispatchState)
	}
}

func TestCreateRoutedKeepsAcceptedSessionState(t *testing.T) {
	store := NewMemoryStore()
	host := &Host{ID: "host-1", PublicID: "100 000 001", Hostname: "demo-host"}
	req := CreateSessionRequest{Target: host.PublicID, Viewer: "viewer-1"}

	first := store.CreateRouted(req, host)
	accepted, err := store.AcknowledgeSessionForHost(host.ID, first.ID, "accept")
	if err != nil {
		t.Fatalf("acknowledge session: %v", err)
	}
	accepted, err = store.SetOffer(accepted.ID, "offer-1")
	if err != nil {
		t.Fatalf("set offer: %v", err)
	}
	accepted, err = store.SetAnswer(accepted.ID, "answer-1")
	if err != nil {
		t.Fatalf("set answer: %v", err)
	}

	second := store.CreateRouted(req, host)
	if second.ID != first.ID {
		t.Fatalf("CreateRouted created %q then %q, want reuse", first.ID, second.ID)
	}
	if second.Status != StatusActive {
		t.Fatalf("status = %q, want active", second.Status)
	}
	if second.OfferSDP != "offer-1" || second.AnswerSDP != "answer-1" {
		t.Fatalf("offer/answer = %q/%q, want preserved", second.OfferSDP, second.AnswerSDP)
	}
	if second.DispatchState != "accepted" {
		t.Fatalf("dispatch state = %q, want accepted", second.DispatchState)
	}
}

func TestSetOfferResetsPreviousAnswerAndRequeuesRoutedSession(t *testing.T) {
	store := NewMemoryStore()
	host := &Host{ID: "host-1", PublicID: "100 000 001", Hostname: "demo-host"}
	session := store.CreateRouted(CreateSessionRequest{Target: host.PublicID, Viewer: "viewer-1"}, host)

	if _, err := store.AcknowledgeSessionForHost(host.ID, session.ID, "accept"); err != nil {
		t.Fatalf("acknowledge session: %v", err)
	}
	if _, err := store.SetAnswer(session.ID, "old-answer"); err != nil {
		t.Fatalf("set answer: %v", err)
	}
	if _, err := store.AddCandidate(session.ID, "old-host-candidate", "host"); err != nil {
		t.Fatalf("add host candidate: %v", err)
	}
	if _, err := store.AddCandidate(session.ID, "old-viewer-candidate", "viewer"); err != nil {
		t.Fatalf("add viewer candidate: %v", err)
	}

	updated, err := store.SetOffer(session.ID, "new-offer")
	if err != nil {
		t.Fatalf("set offer: %v", err)
	}

	if updated.AnswerSDP != "" {
		t.Fatalf("answer SDP = %q, want cleared", updated.AnswerSDP)
	}
	if len(updated.HostICECandidates) != 0 || len(updated.ViewerICECandidates) != 0 {
		t.Fatalf("candidate counts = host %d viewer %d, want 0/0", len(updated.HostICECandidates), len(updated.ViewerICECandidates))
	}
	if updated.DispatchState != "queued" {
		t.Fatalf("dispatch state = %q, want queued", updated.DispatchState)
	}
	if updated.LastHostAckAt != nil {
		t.Fatalf("last host ack = %v, want nil", updated.LastHostAckAt)
	}
}

func TestSetOfferIgnoresDuplicateOffer(t *testing.T) {
	store := NewMemoryStore()
	host := &Host{ID: "host-1", PublicID: "100 000 001", Hostname: "demo-host"}
	session := store.CreateRouted(CreateSessionRequest{Target: host.PublicID, Viewer: "viewer-1"}, host)

	if _, err := store.SetOffer(session.ID, "offer-1"); err != nil {
		t.Fatalf("set offer: %v", err)
	}
	if _, err := store.SetAnswer(session.ID, "answer-1"); err != nil {
		t.Fatalf("set answer: %v", err)
	}
	if _, err := store.AddCandidate(session.ID, "host-candidate-1", "host"); err != nil {
		t.Fatalf("add host candidate: %v", err)
	}
	if _, err := store.AddCandidate(session.ID, "viewer-candidate-1", "viewer"); err != nil {
		t.Fatalf("add viewer candidate: %v", err)
	}

	updated, err := store.SetOffer(session.ID, " offer-1 ")
	if err != nil {
		t.Fatalf("set duplicate offer: %v", err)
	}
	if updated.AnswerSDP != "answer-1" {
		t.Fatalf("answer = %q, want preserved", updated.AnswerSDP)
	}
	if len(updated.HostICECandidates) != 1 || len(updated.ViewerICECandidates) != 1 {
		t.Fatalf("candidate counts = host %d viewer %d, want preserved 1/1", len(updated.HostICECandidates), len(updated.ViewerICECandidates))
	}
}

func TestAddCandidateIgnoresDuplicates(t *testing.T) {
	store := NewMemoryStore()
	session := store.Create(CreateSessionRequest{Target: "host-1", Viewer: "viewer-1"})

	if _, err := store.AddCandidate(session.ID, "candidate-1", "viewer"); err != nil {
		t.Fatalf("add viewer candidate: %v", err)
	}
	updated, err := store.AddCandidate(session.ID, " candidate-1 ", "viewer")
	if err != nil {
		t.Fatalf("add duplicate viewer candidate: %v", err)
	}
	if len(updated.ViewerICECandidates) != 1 {
		t.Fatalf("viewer candidates = %v, want one", updated.ViewerICECandidates)
	}

	if _, err := store.AddCandidate(session.ID, "candidate-2", "host"); err != nil {
		t.Fatalf("add host candidate: %v", err)
	}
	updated, err = store.AddCandidate(session.ID, "candidate-2", "host")
	if err != nil {
		t.Fatalf("add duplicate host candidate: %v", err)
	}
	if len(updated.HostICECandidates) != 1 {
		t.Fatalf("host candidates = %v, want one", updated.HostICECandidates)
	}
}

func assertHostIDs(t *testing.T, hosts []Host, want []string) {
	t.Helper()
	got := make([]string, 0, len(hosts))
	for _, host := range hosts {
		got = append(got, host.ID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("host order = %v, want %v", got, want)
	}
}
