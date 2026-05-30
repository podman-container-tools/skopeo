package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommentFromConfigBlob(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   string
		want    string
	}{
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "malformed JSON",
			input: "{not valid json",
			want:  "",
		},
		{
			name:  "empty object",
			input: `{}`,
			want:  "",
		},
		{
			name:  "neither top-level nor history",
			input: `{"architecture":"amd64","os":"linux","history":[{"created_by":"/bin/sh"}]}`,
			want:  "",
		},
		{
			name:  "top-level comment only",
			input: `{"comment":"top-level msg"}`,
			want:  "top-level msg",
		},
		{
			name:  "history-only single entry",
			input: `{"history":[{"comment":"history msg"}]}`,
			want:  "history msg",
		},
		{
			name:  "history-only multiple entries, only the last has a comment",
			input: `{"history":[{"created_by":"a"},{"created_by":"b"},{"comment":"last layer note"}]}`,
			want:  "last layer note",
		},
		{
			name:  "history-only multiple entries, only the middle has a comment",
			input: `{"history":[{"created_by":"a"},{"comment":"middle note"},{"created_by":"c"}]}`,
			want:  "middle note",
		},
		{
			name:  "history-only multiple entries, several have a comment",
			input: `{"history":[{"comment":"first"},{"comment":"second"},{"comment":"third"}]}`,
			want:  "third",
		},
		{
			name:  "top-level wins over history",
			input: `{"comment":"top","history":[{"comment":"hist"}]}`,
			want:  "top",
		},
		{
			name:  "irrelevant fields don't interfere",
			input: `{"architecture":"arm64","os":"linux","config":{"Env":["PATH=/"]},"comment":"only-top"}`,
			want:  "only-top",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := commentFromConfigBlob([]byte(tc.input))
			assert.Equal(t, tc.want, got)
		})
	}
}
