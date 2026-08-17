package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type findPeopleInput struct{}

type personEntry struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type findPeopleOutput struct {
	People []personEntry `json:"people"`
}

// addFindPeople registers find_people, bound to person: the roster (everyone
// else) as email + name, so the model can resolve "message bob" to an address
// for send_message. The roster is trusted relay state, not message content, so
// it is returned structurally (no spotlight). It enumerates the roster — a
// broader oracle than send_message's probe-one — acceptable for a same-team relay.
func (h *Handler) addFindPeople(s *sdkmcp.Server, person string) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "find_people",
		Description: "List people you can message (email + name). Use this to resolve a name to an email address for send_message.",
		Annotations: &sdkmcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, _ findPeopleInput) (*sdkmcp.CallToolResult, findPeopleOutput, error) {
		people, err := h.store.ListPeople(person)
		if err != nil {
			h.log.Error("find_people: list", "err", err)
			return nil, findPeopleOutput{}, errInternal
		}
		out := findPeopleOutput{People: make([]personEntry, 0, len(people))}
		for _, p := range people {
			out.People = append(out.People, personEntry{Email: p.Email, Name: p.Label})
		}
		return emptyResult(), out, nil
	})
}
