package cloudapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type cloudServer struct {
	ID           string `json:"id"`
	TeamID       string `json:"teamId,omitempty"`
	Name         string `json:"name"`
	Group        string `json:"group,omitempty"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Shell        string `json:"shell"`
	UseTmux      bool   `json:"useTmux,omitempty"`
	TmuxSession  string `json:"tmuxSession,omitempty"`
	JumpServerID string `json:"jumpServerId,omitempty"`
	Fingerprint  string `json:"fingerprint,omitempty"`
	Favorite     bool   `json:"favorite,omitempty"`
}

type cloudTeam struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type cloudTab struct {
	ID          string `json:"id"`
	ServerID    string `json:"serverId"`
	Title       string `json:"title"`
	ManualTitle bool   `json:"manualTitle,omitempty"`
	Restore     bool   `json:"restore,omitempty"`
	LastPath    string `json:"lastPath,omitempty"`
	Position    int    `json:"position,omitempty"`
}

type workspacePatch struct {
	Servers         []cloudServer `json:"servers"`
	Teams           []cloudTeam   `json:"teams"`
	Tabs            []cloudTab    `json:"tabs"`
	DeleteServerIDs []string      `json:"deleteServerIds"`
	DeleteTeamIDs   []string      `json:"deleteTeamIds"`
	DeleteTabIDs    []string      `json:"deleteTabIds"`
}

type workspaceResponse struct {
	Revision  int64         `json:"revision"`
	Servers   []cloudServer `json:"servers"`
	Teams     []cloudTeam   `json:"teams"`
	Tabs      []cloudTab    `json:"tabs"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

type workspaceDocument struct {
	Servers        map[string]cloudServer `json:"servers"`
	Teams          map[string]cloudTeam   `json:"teams"`
	Tabs           map[string]cloudTab    `json:"tabs"`
	DeletedServers map[string]time.Time   `json:"deletedServers,omitempty"`
	DeletedTeams   map[string]time.Time   `json:"deletedTeams,omitempty"`
	DeletedTabs    map[string]time.Time   `json:"deletedTabs,omitempty"`
}

func emptyWorkspaceDocument() workspaceDocument {
	return workspaceDocument{
		Servers: map[string]cloudServer{}, Teams: map[string]cloudTeam{}, Tabs: map[string]cloudTab{},
		DeletedServers: map[string]time.Time{}, DeletedTeams: map[string]time.Time{}, DeletedTabs: map[string]time.Time{},
	}
}

func normalizeDocument(doc *workspaceDocument) {
	if doc.Servers == nil {
		doc.Servers = map[string]cloudServer{}
	}
	if doc.Teams == nil {
		doc.Teams = map[string]cloudTeam{}
	}
	if doc.Tabs == nil {
		doc.Tabs = map[string]cloudTab{}
	}
	if doc.DeletedServers == nil {
		doc.DeletedServers = map[string]time.Time{}
	}
	if doc.DeletedTeams == nil {
		doc.DeletedTeams = map[string]time.Time{}
	}
	if doc.DeletedTabs == nil {
		doc.DeletedTabs = map[string]time.Time{}
	}
}

func validWorkspaceID(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 128
}

func (p *workspacePatch) validate() error {
	if len(p.Servers) > 500 || len(p.Teams) > 100 || len(p.Tabs) > 200 {
		return errors.New("workspace is too large")
	}
	for i := range p.Servers {
		s := &p.Servers[i]
		s.ID = strings.TrimSpace(s.ID)
		s.Name = strings.TrimSpace(s.Name)
		s.Host = strings.TrimSpace(s.Host)
		s.User = strings.TrimSpace(s.User)
		if !validWorkspaceID(s.ID) || s.Name == "" || s.Host == "" || s.User == "" {
			return errors.New("invalid server entry")
		}
		if s.Port < 1 || s.Port > 65535 {
			return errors.New("invalid server port")
		}
	}
	for i := range p.Teams {
		p.Teams[i].ID = strings.TrimSpace(p.Teams[i].ID)
		p.Teams[i].Name = strings.TrimSpace(p.Teams[i].Name)
		if !validWorkspaceID(p.Teams[i].ID) || p.Teams[i].Name == "" {
			return errors.New("invalid team entry")
		}
	}
	for i := range p.Tabs {
		p.Tabs[i].ID = strings.TrimSpace(p.Tabs[i].ID)
		p.Tabs[i].ServerID = strings.TrimSpace(p.Tabs[i].ServerID)
		if !validWorkspaceID(p.Tabs[i].ID) || !validWorkspaceID(p.Tabs[i].ServerID) {
			return errors.New("invalid tab entry")
		}
	}
	for _, ids := range [][]string{p.DeleteServerIDs, p.DeleteTeamIDs, p.DeleteTabIDs} {
		for _, id := range ids {
			if !validWorkspaceID(id) {
				return errors.New("invalid deletion entry")
			}
		}
	}
	return nil
}

func (s *store) workspace(ctx context.Context, userID string) (workspaceResponse, error) {
	var revision int64
	var raw []byte
	var updated time.Time
	err := s.pool.QueryRow(ctx, `SELECT revision,document,updated_at FROM cloud_workspaces WHERE user_id=$1`, userID).Scan(&revision, &raw, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return workspaceResponse{Servers: []cloudServer{}, Teams: []cloudTeam{}, Tabs: []cloudTab{}}, nil
	}
	if err != nil {
		return workspaceResponse{}, err
	}
	doc := emptyWorkspaceDocument()
	if err := json.Unmarshal(raw, &doc); err != nil {
		return workspaceResponse{}, err
	}
	normalizeDocument(&doc)
	return documentResponse(doc, revision, updated), nil
}

func (s *store) syncWorkspace(ctx context.Context, userID string, patch workspacePatch) (workspaceResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return workspaceResponse{}, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO cloud_workspaces(user_id) VALUES($1) ON CONFLICT DO NOTHING`, userID)
	if err != nil {
		return workspaceResponse{}, err
	}
	var revision int64
	var raw []byte
	var updated time.Time
	if err = tx.QueryRow(ctx, `SELECT revision,document,updated_at FROM cloud_workspaces WHERE user_id=$1 FOR UPDATE`, userID).Scan(&revision, &raw, &updated); err != nil {
		return workspaceResponse{}, err
	}
	doc := emptyWorkspaceDocument()
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &doc); err != nil {
			return workspaceResponse{}, err
		}
	}
	normalizeDocument(&doc)
	before, err := json.Marshal(doc)
	if err != nil {
		return workspaceResponse{}, err
	}
	now := time.Now().UTC()
	applyWorkspacePatch(&doc, patch, now)
	encoded, err := json.Marshal(doc)
	if err != nil {
		return workspaceResponse{}, err
	}
	if bytes.Equal(before, encoded) {
		if err = tx.Commit(ctx); err != nil {
			return workspaceResponse{}, err
		}
		return documentResponse(doc, revision, updated), nil
	}
	revision++
	if _, err = tx.Exec(ctx, `UPDATE cloud_workspaces SET revision=$2,document=$3,updated_at=$4 WHERE user_id=$1`, userID, revision, encoded, now); err != nil {
		return workspaceResponse{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return workspaceResponse{}, err
	}
	return documentResponse(doc, revision, now), nil
}

func applyWorkspacePatch(doc *workspaceDocument, patch workspacePatch, now time.Time) {
	for _, id := range patch.DeleteTabIDs {
		delete(doc.Tabs, id)
		doc.DeletedTabs[id] = now
	}
	for _, id := range patch.DeleteServerIDs {
		delete(doc.Servers, id)
		doc.DeletedServers[id] = now
		for tabID, tab := range doc.Tabs {
			if tab.ServerID == id {
				delete(doc.Tabs, tabID)
				doc.DeletedTabs[tabID] = now
			}
		}
	}
	for _, id := range patch.DeleteTeamIDs {
		delete(doc.Teams, id)
		doc.DeletedTeams[id] = now
		for serverID, server := range doc.Servers {
			if server.TeamID == id {
				server.TeamID = ""
				doc.Servers[serverID] = server
			}
		}
	}
	for _, team := range patch.Teams {
		if _, deleted := doc.DeletedTeams[team.ID]; !deleted {
			doc.Teams[team.ID] = team
		}
	}
	for _, server := range patch.Servers {
		if _, deleted := doc.DeletedServers[server.ID]; !deleted {
			doc.Servers[server.ID] = server
		}
	}
	for _, tab := range patch.Tabs {
		if _, deleted := doc.DeletedTabs[tab.ID]; !deleted {
			if _, serverExists := doc.Servers[tab.ServerID]; serverExists {
				doc.Tabs[tab.ID] = tab
			}
		}
	}
}

func documentResponse(doc workspaceDocument, revision int64, updated time.Time) workspaceResponse {
	result := workspaceResponse{Revision: revision, UpdatedAt: updated, Servers: make([]cloudServer, 0, len(doc.Servers)), Teams: make([]cloudTeam, 0, len(doc.Teams)), Tabs: make([]cloudTab, 0, len(doc.Tabs))}
	for _, v := range doc.Servers {
		result.Servers = append(result.Servers, v)
	}
	for _, v := range doc.Teams {
		result.Teams = append(result.Teams, v)
	}
	for _, v := range doc.Tabs {
		result.Tabs = append(result.Tabs, v)
	}
	sort.Slice(result.Servers, func(i, j int) bool { return result.Servers[i].ID < result.Servers[j].ID })
	sort.Slice(result.Teams, func(i, j int) bool { return result.Teams[i].ID < result.Teams[j].ID })
	sort.Slice(result.Tabs, func(i, j int) bool {
		if result.Tabs[i].Position == result.Tabs[j].Position {
			return result.Tabs[i].ID < result.Tabs[j].ID
		}
		return result.Tabs[i].Position < result.Tabs[j].Position
	})
	return result
}

func (s *Server) getWorkspace(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	workspace, err := s.store.workspace(r.Context(), u.ID)
	if err != nil {
		writeError(w, 500, "could not load workspace")
		return
	}
	writeJSON(w, 200, workspace)
}
func (s *Server) syncWorkspace(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var patch workspacePatch
	if !decodeJSON(w, r, &patch) {
		return
	}
	if err := patch.validate(); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	workspace, err := s.store.syncWorkspace(r.Context(), u.ID, patch)
	if err != nil {
		writeError(w, 500, "could not sync workspace")
		return
	}
	writeJSON(w, 200, workspace)
}
