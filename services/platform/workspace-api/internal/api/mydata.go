package api

import (
	"net/http"
	"time"

	"connectrpc.com/connect"

	consentv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1"
	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
)

// myData answers a data subject's access request (POPIA s.23): what Forge holds about the signed-in party, as a file
// they can keep. The identity number is masked, as it is everywhere outside the party service.
func (a *API) myData(w http.ResponseWriter, r *http.Request, s Session) {
	if a.Services.Party == nil || a.Services.Consent == nil {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "data access is not configured"))
		return
	}
	p, err := a.Services.Party.GetParty(r.Context(), connect.NewRequest(&partyv1.GetPartyRequest{PartyId: s.Party}))
	if err != nil {
		writeErr(w, err)
		return
	}
	c, err := a.Services.Consent.List(r.Context(), connect.NewRequest(&consentv1.ListRequest{PartyId: s.Party}))
	if err != nil {
		writeErr(w, err)
		return
	}
	accounts, err := a.balances(r.Context(), s.Party)
	if err != nil {
		writeErr(w, err)
		return
	}
	pt := p.Msg.Party
	consents := []map[string]any{}
	for _, x := range c.Msg.Consents {
		item := map[string]any{"purpose": x.Purpose, "scopes": x.Scopes, "data_sources": x.DataSources, "granted_at": x.GrantedAt.AsTime().Format(time.RFC3339)}
		if x.RevokedAt != nil {
			item["revoked_at"] = x.RevokedAt.AsTime().Format(time.RFC3339)
		}
		consents = append(consents, item)
	}
	w.Header().Set("Content-Disposition", `attachment; filename="my-forge-data.json"`)
	writeJSON(w, http.StatusOK, map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"about_you": map[string]any{
			"party_id": pt.PartyId, "type": pt.Type.String(), "status": pt.Status.String(), "legal_name": pt.LegalName,
			"registration_number": pt.RegistrationNumber, "national_id": pt.NationalIdMasked, "country": pt.Country,
			"customer_since": pt.CreatedAt.AsTime().Format(time.RFC3339),
		},
		"consents": consents,
		"accounts": accounts,
		"also_held": "Transaction history (see Activity on each account); identity verification records at our verification partner; " +
			"records we must keep by law. To correct anything, or for a full copy of these, contact our Information Officer.",
	})
}
