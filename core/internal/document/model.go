package document

import "github.com/duxlabs-dev/dux-edit/core/internal/buffer"

// Result codes shared by every doc.* reply with ok=false.
const (
	CodeNotFound     = "NOTFOUND"
	CodeIsDir        = "ISDIR"
	CodeAccess       = "EACCES"
	CodeBinary       = "BINARY"
	CodeTooLarge     = "TOOLARGE"
	CodeConfirmLarge = "CONFIRM_LARGE"
	CodeLocked       = "LOCKED"
	CodeDesync       = "DESYNC"
	CodeConflict     = "CONFLICT"
	CodeExists       = "EXISTS"
	CodeUnencodable  = "UNENCODABLE"
	CodeIO           = "EIO"
	CodeUnknownDoc   = "NODOC"
)

const (
	ConfirmLargeBytes = 5 << 20
	MaxOpenBytes      = 32 << 20
)

type Failure struct {
	OK        bool   `json:"ok"`
	Code      string `json:"code"`
	Message   string `json:"message,omitempty"`
	Elevation string `json:"elevation,omitempty"`
	Size      int64  `json:"size,omitempty"`
	// Char and Encoding name what an UNENCODABLE save could not write.
	Char     string `json:"char,omitempty"`
	Encoding string `json:"encoding,omitempty"`
}

func Fail(code, message string) Failure { return Failure{Code: code, Message: message} }

type OpenRequest struct {
	Path     string
	SwapID   string
	Encoding string
	Elevated bool
	Force    bool
}

type OpenResult struct {
	OK        bool   `json:"ok"`
	DocID     string `json:"docId"`
	SwapID    string `json:"swapId"`
	Path      string `json:"path"`
	Title     string `json:"title"`
	Text      string `json:"text"`
	Version   int64  `json:"version"`
	Encoding  string `json:"encoding"`
	BOM       bool   `json:"bom"`
	EOL       string `json:"eol"`
	MixedEOL  bool   `json:"mixedEol"`
	ReadOnly  bool   `json:"readOnly"`
	Writable  bool   `json:"writable"`
	Elevated  bool   `json:"elevated"`
	Size      int64  `json:"size"`
	Recovered bool   `json:"recovered"`
	// Recoverable is false for a document read with administrator rights: it has no swap.
	Recoverable bool `json:"recoverable"`
	Dirty       bool `json:"dirty"`
	Watched     bool `json:"watched"`
	// Conflict is "modified" or "deleted" when a recovered swap was based on a file that has since changed.
	Conflict string `json:"conflict"`
}

type SaveRequest struct {
	DocID     string
	Version   int64
	MD5       string
	Path      string
	Encoding  string
	BOM       *bool
	EOL       string
	Elevated  bool
	Force     bool
	Overwrite bool
}

type SaveResult struct {
	OK       bool   `json:"ok"`
	Path     string `json:"path"`
	Title    string `json:"title"`
	Version  int64  `json:"version"`
	Encoding string `json:"encoding"`
	BOM      bool   `json:"bom"`
	EOL      string `json:"eol"`
	Elevated bool   `json:"elevated"`
	// Recoverable turns true when Save As moves a document read as administrator to a file of the user's.
	Recoverable bool `json:"recoverable"`
}

type RecoveryItem struct {
	SwapID  string `json:"swapId"`
	Path    string `json:"path"`
	Title   string `json:"title"`
	SavedAt int64  `json:"savedAt"`
	Size    int64  `json:"size"`
}

// Listener is how highlight and spell follow documents. Calls arrive in version order, outside the doc lock.
type Listener interface {
	DocOpened(d *Doc)
	DocChanged(d *Doc, change buffer.Change)
	DocReset(d *Doc)
	// DocSaved follows every successful save; the path may have changed.
	DocSaved(d *Doc)
	DocClosed(docID string)
}

// Elevator is the polkit path for files the user cannot read or write. internal/elevate implements it.
type Elevator interface {
	// Available returns "" when elevation can be offered, otherwise SANDBOXED, NOPKEXEC, NOPOLICY or NOBINARY.
	Available() string
	// Read, Replace and Create return the file's identity, which the user may be unable to stat.
	Read(path string) ([]byte, string, error)
	// Replace writes data over path only while it still has the identity it was loaded with; "" skips the check.
	Replace(path string, data []byte, identity string) (string, error)
	// Create writes a new file and fails when path exists.
	Create(path string, data []byte) (string, error)
}
