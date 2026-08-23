_⚠️ Potential issue_ | _🟠 Major_

**`head -n 1` closes the pipe and kills `sed` with SIGPIPE**

<!-- an html comment that must be stripped -->

<blockquote>

Under `set -o pipefail` the whole script aborts with nothing printed, which is
the "aborts with no diagnostic" failure mode.

</blockquote>

<details>
<summary>Analysis chain</summary>

machinery: dropped by the denylist

</details>

<details>
<summary>Proposed guard</summary>

use `sed -n 1p` instead

</details>

<details>
<summary>Prompt for AI Agents</summary>

more machinery

</details>
