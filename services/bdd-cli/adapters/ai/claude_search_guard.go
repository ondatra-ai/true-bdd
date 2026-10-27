package ai

import (
	"path/filepath"
	"slices"
	"strings"
)

func (p ClaudeGuardPolicy) allowsGlob(base, pattern string) (bool, string) {
	expanded := expandClaudeHome(pattern)
	if expanded == "" && pattern != "" {
		return false, "cannot resolve the glob pattern"
	}

	pattern = expanded
	if slices.Contains(strings.Split(pattern, "/"), "..") {
		return false, "parent traversal in glob patterns is denied"
	}

	if allowed, reason := p.allowsSearch(base); !allowed {
		return false, reason
	}

	fixed := pattern
	if index := strings.IndexAny(fixed, "*?[{"); index >= 0 {
		fixed = fixed[:index]
		if !strings.HasSuffix(fixed, "/") {
			fixed = filepath.Dir(fixed)
		}
	}

	if !filepath.IsAbs(fixed) {
		fixed = base + string(filepath.Separator) + fixed
	}

	return p.allowsSearch(fixed)
}

func (p ClaudeGuardPolicy) allowsSearch(target string) (bool, string) {
	if allowed, reason := p.allowsRead(target); !allowed {
		return false, reason
	}

	resolved := resolveThroughSymlinks(target)

	cache := resolveThroughSymlinks(p.LibraryCache)
	if p.LibraryCache != "" && claudePathWithinRoot(cache, resolved) &&
		!claudePathWithinRoot(resolveThroughSymlinks(p.Replay.ReadRoot), cache) {
		return false, "search root contains the module cache; choose a declared library or narrower host directory"
	}

	if claudePathWithinRoot(resolved, resolveThroughSymlinks(p.Replay.ReadRoot)) {
		return true, ""
	}

	for _, root := range p.ExcludedRoots {
		if claudePathWithinRoot(resolveThroughSymlinks(root), resolved) {
			return false, "search root contains excluded module copies; choose a narrower dependency directory"
		}
	}

	return true, ""
}
