package aishell

import (
	"fmt"
	"sort"
	"strings"
)

type ManagedInstance struct {
	Workdir    string
	InstanceID string
	Container  string
	Status     string
	Image      string
	Volume     string
}

func listManagedInstances(d Docker) ([]ManagedInstance, error) {
	names, err := d.PSNamesByLabel(LabelManaged, "true")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)

	var out []ManagedInstance
	for _, name := range names {
		info, err := d.InspectContainer(name)
		if err != nil {
			continue
		}
		// Discover workdir from /work bind mount (single source of truth).
		wd := info.Workdir()
		var iid, vol string
		if info.Config.Labels != nil {
			iid = info.Config.Labels[LabelInstance]
			vol = info.Config.Labels[LabelVolume]
		}
		out = append(out, ManagedInstance{
			Workdir:    wd,
			InstanceID: iid,
			Container:  name,
			Status:     info.State.Status,
			Image:      info.Config.Image,
			Volume:     vol,
		})
	}
	return out, nil
}

func resolveTarget(d Docker, target string) (ManagedInstance, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return ManagedInstance{}, fmt.Errorf("target must not be empty")
	}
	instances, err := listManagedInstances(d)
	if err != nil {
		return ManagedInstance{}, err
	}
	if len(instances) == 0 {
		return ManagedInstance{}, fmt.Errorf("no ai-shell managed containers found (run: ai-shell up)")
	}
	return matchTarget(target, instances)
}

func matchTarget(target string, instances []ManagedInstance) (ManagedInstance, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return ManagedInstance{}, fmt.Errorf("target must not be empty")
	}

	// 1) Exact container name match.
	if m := filter(instances, func(i ManagedInstance) bool { return i.Container == target }); len(m) == 1 {
		return m[0], nil
	} else if len(m) > 1 {
		return ManagedInstance{}, ambiguousTargetErr(target, m)
	}

	// 2) Exact instance id match (label com.nimzi.ai-shell.instance).
	if m := filter(instances, func(i ManagedInstance) bool { return i.InstanceID == target }); len(m) == 1 {
		return m[0], nil
	} else if len(m) > 1 {
		return ManagedInstance{}, ambiguousTargetErr(target, m)
	}

	// 3) Unique instance id prefix match.
	if m := filter(instances, func(i ManagedInstance) bool {
		return i.InstanceID != "" && strings.HasPrefix(i.InstanceID, target)
	}); len(m) == 1 {
		return m[0], nil
	} else if len(m) > 1 {
		return ManagedInstance{}, ambiguousTargetErr(target, m)
	}

	// 4) Unique container name prefix match.
	if m := filter(instances, func(i ManagedInstance) bool { return strings.HasPrefix(i.Container, target) }); len(m) == 1 {
		return m[0], nil
	} else if len(m) > 1 {
		return ManagedInstance{}, ambiguousTargetErr(target, m)
	}

	// 5) Workdir match (path-like only).
	if looksLikePath(target) {
		wd, err := CanonicalWorkdir(target)
		if err != nil {
			// If canonicalization fails (dir missing), match by exact label value.
			wd = target
		}
		if m := filter(instances, func(i ManagedInstance) bool { return i.Workdir == wd }); len(m) == 1 {
			return m[0], nil
		} else if len(m) > 1 {
			return ManagedInstance{}, ambiguousTargetErr(target, m)
		}
	}

	return ManagedInstance{}, fmt.Errorf("no managed container matches target %q (run: ai-shell ls)", target)
}

func looksLikePath(s string) bool {
	// Heuristic: accept obvious paths; avoid treating short prefixes as paths.
	return strings.Contains(s, "/") || strings.HasPrefix(s, ".") || strings.HasPrefix(s, "~")
}

func ambiguousTargetErr(target string, candidates []ManagedInstance) error {
	return fmt.Errorf("ambiguous target %q; candidates:\n%s", target, formatCandidates(candidates))
}

func formatCandidates(candidates []ManagedInstance) string {
	type cand struct {
		iid       string
		container string
		workdir   string
	}
	var cs []cand
	for _, c := range candidates {
		cs = append(cs, cand{iid: c.InstanceID, container: c.Container, workdir: c.Workdir})
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].container != cs[j].container {
			return cs[i].container < cs[j].container
		}
		if cs[i].iid != cs[j].iid {
			return cs[i].iid < cs[j].iid
		}
		return cs[i].workdir < cs[j].workdir
	})
	var b strings.Builder
	for _, c := range cs {
		// Container name is usually the most actionable.
		b.WriteString("  - ")
		if c.iid != "" {
			b.WriteString(c.iid)
			b.WriteString("  ")
		}
		b.WriteString(c.container)
		if c.workdir != "" {
			b.WriteString("  ")
			b.WriteString(c.workdir)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func filter[T any](in []T, pred func(T) bool) []T {
	var out []T
	for _, v := range in {
		if pred(v) {
			out = append(out, v)
		}
	}
	return out
}

// uniquePrefixLen returns the minimal prefix length N such that all ids have a
// unique prefix of length N, bounded to [min, max]. If no such N exists (e.g.
// duplicate ids), it returns 0.
func uniquePrefixLen(ids []string, min, max int) int {
	if min < 1 || max < min {
		return 0
	}
	for n := min; n <= max; n++ {
		seen := map[string]int{}
		ok := true
		for _, id := range ids {
			if len(id) < n {
				ok = false
				break
			}
			p := id[:n]
			seen[p]++
			if seen[p] > 1 {
				ok = false
				break
			}
		}
		if ok {
			return n
		}
	}
	return 0
}
