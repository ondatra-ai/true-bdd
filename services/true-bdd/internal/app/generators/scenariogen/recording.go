package scenariogen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func quoteStrings(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, strconv.Quote(value))
	}

	return strings.Join(quoted, ", ")
}

func renderDependencies(plan *Plan, serviceName string) string {
	var rendered strings.Builder

	for _, service := range plan.Services {
		if service.Name != serviceName {
			continue
		}

		for _, dependency := range service.CLIDependencies() {
			fmt.Fprintf(&rendered, "{Name:%q, Recording:proxycli.Recording{Placeholders:[]proxycli.Placeholder{",
				dependency.Binary())

			for _, placeholder := range dependency.Recording.Placeholders {
				fmt.Fprintf(&rendered, "{Token:%q,Pattern:%q,Directory:%q},",
					placeholder.Token, placeholder.Pattern, placeholder.Directory)
			}

			fmt.Fprintf(&rendered, "},Unrecorded:[]string{%s},Unread:[]string{%s},ReadOnlyWhen:map[string]string{",
				quoteStrings(dependency.Recording.Unrecorded), quoteStrings(dependency.Recording.Unread))

			keys := make([]string, 0, len(dependency.Recording.ReadOnlyWhen))
			for key := range dependency.Recording.ReadOnlyWhen {
				keys = append(keys, key)
			}

			sort.Strings(keys)

			for _, key := range keys {
				fmt.Fprintf(&rendered, "%q:%q,", key, dependency.Recording.ReadOnlyWhen[key])
			}

			rendered.WriteString("}}},")
		}
	}

	return rendered.String()
}

func renderRecordExclude(exclude []string) string {
	if len(exclude) == 0 {
		return ""
	}

	return fmt.Sprintf(", RecordExclude: []string{%s}", quoteStrings(exclude))
}
