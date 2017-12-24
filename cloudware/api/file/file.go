package file

import (
	"bytes"
	"io/ioutil"

	"io"
	"os"
	"path"

	"cloudware/cloudware/api"
)

const (
	// TLSStorePath represents the subfolder where TLS files are stored in the file store folder.
	TLSStorePath = "tls"
	// LDAPStorePath represents the subfolder where LDAP TLS files are stored in the TLSStorePath.
	LDAPStorePath = "ldap"
	// TLSCACertFile represents the name on disk for a TLS CA file.
	TLSCACertFile = "ca.pem"
	// TLSCertFile represents the name on disk for a TLS certificate file.
	TLSCertFile = "cert.pem"
	// TLSKeyFile represents the name on disk for a TLS key file.
	TLSKeyFile = "key.pem"
	// ComposeStorePath represents the subfolder where compose files are stored in the file store folder.
	ComposeStorePath = "compose"
	// ComposeFileDefaultName represents the default name of a compose file.
	ComposeFileDefaultName = "docker-compose.yml"
)

// Service represents a service for managing files and directories.
type Service struct {
	dataStorePath string
	fileStorePath string
}

// NewService initializes a new service. It creates a data directory and a directory to store files
// inside this directory if they don't exist.
func NewService(dataStorePath, fileStorePath string) (*Service, error) {
	service := &Service{
		dataStorePath: dataStorePath,
		fileStorePath: path.Join(dataStorePath, fileStorePath),
	}

	// Checking if a mount directory exists is broken with Go on Windows.
	// This will need to be reviewed after the issue has been fixed in Go.
	// See: https://github.com/api/api/issues/474
	// err := createDirectoryIfNotExist(dataStorePath, 0755)
	// if err != nil {
	// 	return nil, err
	// }

	err := service.createDirectoryInStoreIfNotExist(TLSStorePath)
	if err != nil {
		return nil, err
	}

	err = service.createDirectoryInStoreIfNotExist(ComposeStorePath)
	if err != nil {
		return nil, err
	}

	return service, nil
}

// RemoveDirectory removes a directory on the filesystem.
func (service *Service) RemoveDirectory(directoryPath string) error {
	return os.RemoveAll(directoryPath)
}

// GetStackProjectPath returns the absolute path on the FS for a stack based
// on its identifier.
func (service *Service) GetStackProjectPath(stackIdentifier string) string {
	return path.Join(service.fileStorePath, ComposeStorePath, stackIdentifier)
}

// StoreStackFileFromString creates a subfolder in the ComposeStorePath and stores a new file using the content from a string.
// It returns the path to the folder where the file is stored.
func (service *Service) StoreStackFileFromString(stackIdentifier, stackFileContent string) (string, error) {
	stackStorePath := path.Join(ComposeStorePath, stackIdentifier)
	err := service.createDirectoryInStoreIfNotExist(stackStorePath)
	if err != nil {
		return "", err
	}

	composeFilePath := path.Join(stackStorePath, ComposeFileDefaultName)
	data := []byte(stackFileContent)
	r := bytes.NewReader(data)

	err = service.createFileInStore(composeFilePath, r)
	if err != nil {
		return "", err
	}

	return path.Join(service.fileStorePath, stackStorePath), nil
}

// StoreStackFileFromReader creates a subfolder in the ComposeStorePath and stores a new file using the content from an io.Reader.
// It returns the path to the folder where the file is stored.
func (service *Service) StoreStackFileFromReader(stackIdentifier string, r io.Reader) (string, error) {
	stackStorePath := path.Join(ComposeStorePath, stackIdentifier)
	err := service.createDirectoryInStoreIfNotExist(stackStorePath)
	if err != nil {
		return "", err
	}

	composeFilePath := path.Join(stackStorePath, ComposeFileDefaultName)

	err = service.createFileInStore(composeFilePath, r)
	if err != nil {
		return "", err
	}

	return path.Join(service.fileStorePath, stackStorePath), nil
}

// StoreTLSFile creates a folder in the TLSStorePath and stores a new file with the content from r.
func (service *Service) StoreTLSFile(folder string, fileType api.TLSFileType, r io.Reader) error {
	storePath := path.Join(TLSStorePath, folder)
	err := service.createDirectoryInStoreIfNotExist(storePath)
	if err != nil {
		return err
	}

	var fileName string
	switch fileType {
	case api.TLSFileCA:
		fileName = TLSCACertFile
	case api.TLSFileCert:
		fileName = TLSCertFile
	case api.TLSFileKey:
		fileName = TLSKeyFile
	default:
		return api.ErrUndefinedTLSFileType
	}

	tlsFilePath := path.Join(storePath, fileName)
	err = service.createFileInStore(tlsFilePath, r)
	if err != nil {
		return err
	}
	return nil
}

// GetPathForTLSFile returns the absolute path to a specific TLS file for an endpoint.
func (service *Service) GetPathForTLSFile(folder string, fileType api.TLSFileType) (string, error) {
	var fileName string
	switch fileType {
	case api.TLSFileCA:
		fileName = TLSCACertFile
	case api.TLSFileCert:
		fileName = TLSCertFile
	case api.TLSFileKey:
		fileName = TLSKeyFile
	default:
		return "", api.ErrUndefinedTLSFileType
	}
	return path.Join(service.fileStorePath, TLSStorePath, folder, fileName), nil
}

// DeleteTLSFiles deletes a folder in the TLS store path.
func (service *Service) DeleteTLSFiles(folder string) error {
	storePath := path.Join(service.fileStorePath, TLSStorePath, folder)
	err := os.RemoveAll(storePath)
	if err != nil {
		return err
	}
	return nil
}

// DeleteTLSFile deletes a specific TLS file from a folder.
func (service *Service) DeleteTLSFile(folder string, fileType api.TLSFileType) error {
	var fileName string
	switch fileType {
	case api.TLSFileCA:
		fileName = TLSCACertFile
	case api.TLSFileCert:
		fileName = TLSCertFile
	case api.TLSFileKey:
		fileName = TLSKeyFile
	default:
		return api.ErrUndefinedTLSFileType
	}

	filePath := path.Join(service.fileStorePath, TLSStorePath, folder, fileName)

	err := os.Remove(filePath)
	if err != nil {
		return err
	}
	return nil
}

// GetFileContent returns a string content from file.
func (service *Service) GetFileContent(filePath string) (string, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// createDirectoryInStoreIfNotExist creates a new directory in the file store if it doesn't exists on the file system.
func (service *Service) createDirectoryInStoreIfNotExist(name string) error {
	path := path.Join(service.fileStorePath, name)
	return createDirectoryIfNotExist(path, 0700)
}

// createDirectoryIfNotExist creates a directory if it doesn't exists on the file system.
func createDirectoryIfNotExist(path string, mode uint32) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		err = os.Mkdir(path, os.FileMode(mode))
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

// createFile creates a new file in the file store with the content from r.
func (service *Service) createFileInStore(filePath string, r io.Reader) error {
	path := path.Join(service.fileStorePath, filePath)

	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, r)
	if err != nil {
		return err
	}

	return nil
}
