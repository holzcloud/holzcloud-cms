package activity

// The actions of the media library and the albums. Written by the AI tools,
// which have no screen behind them where an operator could see what happened —
// the log is the only record of it. In a file of their own so the list in
// entry.go is not the one place every area has to edit at once.
const (
	ActionMediaUpload  = "media.upload"
	ActionMediaUpdate  = "media.update"
	ActionMediaCrop    = "media.crop"
	ActionMediaDelete  = "media.delete"
	ActionAlbumCreate  = "album.create"
	ActionAlbumRename  = "album.rename"
	ActionAlbumDelete  = "album.delete"
	ActionAlbumPicture = "album.picture"
)
