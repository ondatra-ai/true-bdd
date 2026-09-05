package steps

// Literals more than two step definitions repeat, named once so a rename
// cannot miss a copy and goconst has one place to point at.
const (
	architectureNode = "architecture"
	productNode      = "product"

	bodyKey        = "body"
	folderKey      = "folder"
	sessionIDField = "session_id"
	statusKey      = "status"

	// noneWord is what a clause writes where a list is empty.
	noneWord = "none"
)
