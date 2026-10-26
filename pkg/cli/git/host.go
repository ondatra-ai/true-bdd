package git

import "context"

// InitHost makes an installed sample host a repository without user templates or hooks.
func (r Repository) InitHost(ctx context.Context) error {
	_, err := r.output(ctx, "init", "--template=")

	return err
}
