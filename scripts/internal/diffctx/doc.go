// Package diffctx builds the bounded diff context for a `claude -p` turn.
//
// It exists because PR #70 was 276 files and 3.45 MB of diff. Both the PR
// updater and the commit-message step piped that whole thing to `claude -p`,
// which answered "Prompt is too long" — no PR, no commit message, and
// (because the output was redirected to a file) no visible error either.
// See ClickUp 86cb6g6q8.
//
// The shape of the fix: the --stat is small, structured, and carries the full
// scope of the change, so it always goes in complete. The diff BODY is what
// has to be bounded. Excluding the bulk-by-nature paths is not enough on its
// own — on #70 they take 3.45 MB down to 1.03 MB, still far past the limit —
// so the byte cap is the load-bearing part and the exclusions just spend the
// budget on files that carry signal.
//
// A model told it is looking at a prefix leans on the stat; one that is not
// told will describe the prefix as if it were the whole change, which is why
// the shape is named in the text the model reads.
package diffctx
