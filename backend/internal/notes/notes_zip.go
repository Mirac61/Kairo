package notes

import (
	"archive/zip"
	"io"
	"io/fs"
	"path"
	"strings"
)

// Zip schreibt den ganzen Notizordner als ZIP nach w (Notizen, PDFs, Bilder,
// Zeichnungen). Versteckte Einträge (.trash, .obsidian, .git, …) bleiben außen vor.
func (s *Store) Zip(w io.Writer) error {
	zw := zip.NewWriter(w)
	err := fs.WalkDir(s.root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case p != "." && strings.HasPrefix(d.Name(), "."):
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		case d.IsDir():
			return nil
		}
		fi, err := d.Info()
		if err != nil || !fi.Mode().IsRegular() {
			return nil // Symlinks und Sonderdateien auslassen
		}
		f, err := s.root.Open(p)
		if err != nil {
			return nil // während des Laufs gelöscht: überspringen
		}
		defer f.Close()
		hdr, err := zip.FileInfoHeader(fi)
		if err != nil {
			return err
		}
		hdr.Name = path.Clean(p)
		hdr.Method = zip.Deflate
		wf, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		_, err = io.Copy(wf, f)
		return err
	})
	if err != nil {
		return err
	}
	return zw.Close()
}
