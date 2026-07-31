package cloudapi

import (
	"testing"
	"time"
)

func TestWorkspacePatchRejectsInvalidOrSecretlessServerFields(t *testing.T) {
	valid := workspacePatch{Servers: []cloudServer{{ID: "server-1", Name: "Production", Host: "example.com", Port: 22, User: "ubuntu"}}}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid workspace rejected: %v", err)
	}
	invalid := valid
	invalid.Servers[0].Port = 70000
	if err := invalid.validate(); err == nil {
		t.Fatal("invalid port accepted")
	}
}

func TestWorkspaceDocumentInitializesMaps(t *testing.T) {
	var doc workspaceDocument
	normalizeDocument(&doc)
	if doc.Servers == nil || doc.Teams == nil || doc.Tabs == nil || doc.DeletedServers == nil || doc.DeletedTabs == nil {
		t.Fatal("workspace maps were not initialized")
	}
}

func TestWorkspaceDeletionTombstonePreventsStaleResurrection(t *testing.T) {
	doc := emptyWorkspaceDocument()
	server := cloudServer{ID: "server-1", Name: "Production", Host: "example.com", Port: 22, User: "ubuntu"}
	tab := cloudTab{ID: "tab-1", ServerID: "server-1", Title: "top"}
	applyWorkspacePatch(&doc, workspacePatch{Servers: []cloudServer{server}, Tabs: []cloudTab{tab}}, time.Now())
	applyWorkspacePatch(&doc, workspacePatch{DeleteTabIDs: []string{"tab-1"}}, time.Now())
	applyWorkspacePatch(&doc, workspacePatch{Tabs: []cloudTab{tab}}, time.Now())
	if _, exists := doc.Tabs["tab-1"]; exists {
		t.Fatal("stale device resurrected a deleted tab")
	}
	if _, exists := doc.DeletedTabs["tab-1"]; !exists {
		t.Fatal("tab tombstone missing")
	}
}
