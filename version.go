package main

import (
	"encoding/json"
	"fmt"
	"github.com/gabibotos/go-srv/log"
	"net/http"
)

var (
	Version   string
	LastBuild string
	GitCommit string
	GitState  string
)

type VersionInfo struct {
	Version   string `json:"version,omitempty"`
	LastBuild string `json:"lastBuild,omitempty"`
	GitCommit string `json:"gitCommit,omitempty"`
	GitState  string `json:"gitState,omitempty"`
}

func NewVersionInfo() VersionInfo {
	ver := VersionInfo{
		Version:   "dev",
		LastBuild: LastBuild,
		GitCommit: GitCommit,
		GitState:  "",
	}
	if Version != "" {
		ver.Version = Version
		ver.GitState = "clean"
	}
	if GitState != "" {
		ver.GitState = GitState
	}
	return ver
}

func (v VersionInfo) String() string {
	return fmt.Sprintf("Version: %s\nBuild date: %s\nCommit: %s\nWorking tree: %s\n",
		v.Version, v.LastBuild, v.GitCommit, v.GitState)
}

func VersionHandler(log log.Logger, info VersionInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json;charset=utf-8")
		enc := json.NewEncoder(w)
		if err := enc.Encode(info); err != nil {
			log.Printf("failed to write version response: %v", err)
		}
	}
}
