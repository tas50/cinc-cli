package nodeedit

import (
	"strings"
	"testing"

	cinc "github.com/tas50/cinc-api"
)

func TestRunListRoundTrip(t *testing.T) {
	n := &cinc.Node{RunList: []string{"recipe[nginx]", "role[base]"}}
	if got := runListText(n); got != "recipe[nginx]\nrole[base]" {
		t.Errorf("runListText = %q", got)
	}
	got := parseRunList("recipe[nginx]\n  role[base]  \n\n")
	want := []string{"recipe[nginx]", "role[base]"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("parseRunList = %v, want %v", got, want)
	}
}

func TestAttributesSeedShowsAllFourBagsInOrder(t *testing.T) {
	n := &cinc.Node{Normal: cinc.Attributes{"port": 5432}}
	seed, err := attributesSeed(n)
	if err != nil {
		t.Fatalf("attributesSeed: %v", err)
	}
	s := string(seed)
	iNormal := strings.Index(s, "normal")
	iDefault := strings.Index(s, "default")
	iOverride := strings.Index(s, "override")
	iAutomatic := strings.Index(s, "automatic")
	if !(iNormal >= 0 && iNormal < iDefault && iDefault < iOverride && iOverride < iAutomatic) {
		t.Errorf("bags out of order in seed:\n%s", s)
	}
	if !strings.Contains(s, "5432") {
		t.Errorf("normal attribute missing from seed:\n%s", s)
	}
}

func TestBuildNodeAssemblesFields(t *testing.T) {
	orig := &cinc.Node{Name: "db01", Automatic: cinc.Attributes{"platform": "ubuntu"}}
	attrs := []byte(`{"normal":{"role":"db"},"default":{},"override":{},"automatic":{"platform":"ubuntu"}}`)
	n, err := buildNode(orig, "prod", "base", "prodgrp", []string{"recipe[x]"}, attrs)
	if err != nil {
		t.Fatalf("buildNode: %v", err)
	}
	if n.Name != "db01" {
		t.Errorf("name = %q, want db01 (carried from original)", n.Name)
	}
	if n.Environment != "prod" || n.PolicyName != "base" || n.PolicyGroup != "prodgrp" {
		t.Errorf("fields = %+v", n)
	}
	if len(n.RunList) != 1 || n.RunList[0] != "recipe[x]" {
		t.Errorf("run_list = %v", n.RunList)
	}
	if n.Normal["role"] != "db" || n.Automatic["platform"] != "ubuntu" {
		t.Errorf("attributes = normal:%v automatic:%v", n.Normal, n.Automatic)
	}
}

func TestBuildNodeRejectsUnknownAttributeBag(t *testing.T) {
	orig := &cinc.Node{Name: "db01"}
	attrs := []byte(`{"normal":{},"default":{},"override":{},"automatic":{},"bogus":{}}`)
	if _, err := buildNode(orig, "", "", "", nil, attrs); err == nil {
		t.Error("expected an error for an unknown attribute bag")
	}
}

func TestNodeUnchangedSignature(t *testing.T) {
	orig := &cinc.Node{Name: "db01", Environment: "prod"}
	same := &cinc.Node{Name: "db01", Environment: "prod", RunList: []string{}, Normal: cinc.Attributes{}}
	if !nodeUnchanged(orig, same) {
		t.Error("nil and empty collections should compare as unchanged")
	}
	changed := &cinc.Node{Name: "db01", Environment: "staging"}
	if nodeUnchanged(orig, changed) {
		t.Error("an environment change should compare as changed")
	}
}
