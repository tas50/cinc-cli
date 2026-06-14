package main

import (
	"strings"
	"testing"
)

func TestRenderExamples(t *testing.T) {
	t.Run("prose becomes paragraphs and commands become code blocks", func(t *testing.T) {
		got := renderExamples("Create a node with a run-list.\ncinc node create web01 --run-list 'recipe[base]'")
		want := "### Examples\n\n" +
			"Create a node with a run-list.\n\n" +
			"```\ncinc node create web01 --run-list 'recipe[base]'\n```\n\n"
		if got != want {
			t.Errorf("renderExamples =\n%q\nwant\n%q", got, want)
		}
	})

	t.Run("multiple prose+command blocks", func(t *testing.T) {
		got := renderExamples("Show every node.\ncinc node status\n\nLimit to a query.\ncinc node status 'role:web'")
		for _, frag := range []string{
			"Show every node.\n\n```\ncinc node status\n```",
			"Limit to a query.\n\n```\ncinc node status 'role:web'\n```",
		} {
			if !strings.Contains(got, frag) {
				t.Errorf("missing fragment %q in\n%s", frag, got)
			}
		}
		// Two separate fenced blocks (four ``` lines), not one merged block.
		if strings.Count(got, "```") != 4 {
			t.Errorf("want two code fences (4 backtick lines), got %d in\n%s", strings.Count(got, "```"), got)
		}
	})

	t.Run("consecutive command lines share one code block", func(t *testing.T) {
		got := renderExamples("Do two things.\ncinc node list\ncinc role list")
		if strings.Count(got, "```") != 2 {
			t.Errorf("consecutive commands should share one fenced block; got %d fences in\n%s", strings.Count(got, "```"), got)
		}
		if !strings.Contains(got, "```\ncinc node list\ncinc role list\n```") {
			t.Errorf("commands not grouped:\n%s", got)
		}
	})
}
