package redfishwrapper

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	rf "github.com/stmcginnis/gofish/redfish"
)

// Set the virtual media attached to the system, or just eject everything if mediaURL is empty.
func (c *Client) SetVirtualMedia(ctx context.Context, kind string, mediaURL string, username string, password string) (bool, error) {
	managers, err := c.Managers(ctx)
	if err != nil {
		return false, err
	}
	if len(managers) == 0 {
		return false, errors.New("no redfish managers found")
	}

	var mediaKind rf.VirtualMediaType
	switch kind {
	case "CD":
		mediaKind = rf.CDMediaType
	case "Floppy":
		mediaKind = rf.FloppyMediaType
	case "USBStick":
		mediaKind = rf.USBStickMediaType
	case "DVD":
		mediaKind = rf.DVDMediaType
	default:
		return false, errors.New("invalid media type")
	}

	for _, m := range managers {
		virtualMedia, err := m.VirtualMedia()
		if err != nil {
			return false, err
		}
		if len(virtualMedia) == 0 {
			return false, errors.New("no virtual media found")
		}

		for _, vm := range virtualMedia {
			var ejected bool
			if vm.Inserted && vm.SupportsMediaEject {
				if err := vm.EjectMedia(); err != nil {
					return false, err
				}
				ejected = true
			}
			if mediaURL == "" {
				// Only ejecting the media was requested.
				// For BMC's that don't support the "inserted" property, we need to eject the media if it's not already ejected.
				if !ejected && vm.SupportsMediaEject {
					if err := vm.EjectMedia(); err != nil {
						return false, err
					}
				}
				return true, nil
			}
			if !slices.Contains(vm.MediaTypes, mediaKind) {
				return false, fmt.Errorf("media kind %s not supported by BMC, supported media kinds %q", kind, vm.MediaTypes)
			}

			// Determine the transfer protocol type based on the mediaURL
			transferProtocolType := determineTransferProtocolType(mediaURL)

			// Create the VirtualMediaConfig
			config := rf.VirtualMediaConfig{
				Image:                mediaURL,
				Inserted:             true,
				WriteProtected:       true,
				TransferProtocolType: transferProtocolType,
				UserName:             username,
				Password:             password,
			}

			// Try inserting media with the config
			err = vm.InsertMediaConfig(config)
			if err != nil {
				return false, fmt.Errorf("failed to insert media: %w", err)
			}
			return true, nil
		}
	}

	// If we actually get here, then something very unexpected happened as there isn't a known code path that would cause this error to be returned.
	return false, errors.New("unexpected error setting virtual media")
}

// determineTransferProtocolType tries to determine the transfer protocol type based on the URL
func determineTransferProtocolType(url string) rf.TransferProtocolType {
	switch {
	case strings.HasPrefix(url, "http://"):
		return rf.HTTPTransferProtocolType
	case strings.HasPrefix(url, "https://"):
		return rf.HTTPSTransferProtocolType
	case strings.HasPrefix(url, "nfs://"):
		return rf.NFSTransferProtocolType
	case strings.HasPrefix(url, "ftp://"):
		return rf.FTPTransferProtocolType
	case strings.HasPrefix(url, "sftp://"):
		return rf.SFTPTransferProtocolType
	case strings.HasPrefix(url, "cifs://"):
		return rf.CIFSTransferProtocolType
	default:
		// Default to HTTP if unable to determine
		return rf.HTTPTransferProtocolType
	}
}

func (c *Client) InsertedVirtualMedia(ctx context.Context) ([]string, error) {
	managers, err := c.Managers(ctx)
	if err != nil {
		return nil, err
	}

	var inserted []string

	for _, m := range managers {
		virtualMedia, err := m.VirtualMedia()
		if err != nil {
			return nil, err
		}

		for _, media := range virtualMedia {
			if media.Inserted {
				inserted = append(inserted, media.ID)
			}
		}
	}

	return inserted, nil
}
