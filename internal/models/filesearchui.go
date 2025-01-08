package models

import "fyne.io/fyne/v2/data/binding"

type FilesearchUI struct {
	FileCount           binding.String
	AcceptedFiles       binding.String
	GetFileInfoProgress binding.Float
	GenPHashesProgress  binding.Float
}
