**Actionable comments posted: 2**

<details>
<summary>🧹 Nitpick comments (3)</summary><blockquote>

<details>
<summary>services/foo/bar.go (2)</summary><blockquote>

`12-15`: **Guard the nil dereference**

_🟠 Major_

`cfg` can be nil when the loader skips an absent file, and line 14 reaches
through it without checking.

<details>
<summary>Analysis chain</summary>

This whole block is machinery and must be dropped: it is the bot narrating
its own verification run.

</details>

<details>
<summary>Committable suggestion</summary>

```go
if cfg == nil {
    return ErrNoConfig
}
```

</details>

<!-- cr-comment:v1:abc123def456 -->

---

`40-44`: **Second finding in the same file**

_🟡 Minor_

The error message names a path that `$HERE` already holds.

<details>
<summary>Prompt for AI Agents</summary>

noise: this summary is on the denylist too

</details>

<!-- cr-comment:v1:beef00420042 -->

---

trailing footer text after the last marker, which is NOT a finding

</blockquote></details>

<details>
<summary>docs/reference.md (1)</summary><blockquote>

`7-7`: **A documentation nit with nesting**

_Trivial_

<details>
<summary>Proposed change</summary>

<details>
<summary>Inner block three deep</summary>

deeply nested content that must survive

</details>

outer content after the inner block

</details>

<!-- cr-comment:v1:cafe12345678 -->

</blockquote></details>

</blockquote></details>

<details>
<summary>🔭 Outside diff range comments (1)</summary><blockquote>

<details>
<summary>tests/bdd-cli/runner.go (1)</summary><blockquote>

`88-90`: **A finding with no severity label at all**

Plain prose, no bold marker on the first line and no underscore severity.

<!-- cr-comment:v1:0123456789ab -->

</blockquote></details>

</blockquote></details>

<details>
<summary>📜 Review details</summary>

Files selected for processing (3)

this entire block is noise and must not appear anywhere in the output

</details>
