package common

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// AuthCommand implements `tctl auth` group of commands
type AuthCommand struct {
	config                     *service.Config
	authType                   string
	genPubPath                 string
	genPrivPath                string
	genUser                    string
	genTTL                     time.Duration
	exportAuthorityFingerprint string
	exportPrivateKeys          bool
	output                     string
	outputFormat               client.IdentityFileFormat
	compatVersion              string
	compatibility              string
}

// ExportAuthorities outputs the list of authorities in OpenSSH compatible formats
// If --type flag is given, only prints keys for CAs of this type, otherwise
// prints all keys
func (a *AuthCommand) ExportAuthorities(client *auth.TunClient) error {
	var typesToExport []services.CertAuthType

	// if no --type flag is given, export all types
	if a.authType == "" {
		typesToExport = []services.CertAuthType{services.HostCA, services.UserCA}
	} else {
		authType := services.CertAuthType(a.authType)
		if err := authType.Check(); err != nil {
			return trace.Wrap(err)
		}
		typesToExport = []services.CertAuthType{authType}
	}
	localAuthName, err := client.GetDomainName()
	if err != nil {
		return trace.Wrap(err)
	}

	// fetch authorities via auth API (and only take local CAs, ignoring
	// trusted ones)
	var authorities []services.CertAuthority
	for _, at := range typesToExport {
		cas, err := client.GetCertAuthorities(at, a.exportPrivateKeys)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, ca := range cas {
			if ca.GetClusterName() == localAuthName {
				authorities = append(authorities, ca)
			}
		}
	}

	// print:
	for _, ca := range authorities {
		if a.exportPrivateKeys {
			for _, key := range ca.GetSigningKeys() {
				fingerprint, err := sshutils.PrivateKeyFingerprint(key)
				if err != nil {
					return trace.Wrap(err)
				}
				if a.exportAuthorityFingerprint != "" && fingerprint != a.exportAuthorityFingerprint {
					continue
				}
				os.Stdout.Write(key)
				fmt.Fprintf(os.Stdout, "\n")
			}
		} else {
			for _, keyBytes := range ca.GetCheckingKeys() {
				fingerprint, err := sshutils.AuthorizedKeyFingerprint(keyBytes)
				if err != nil {
					return trace.Wrap(err)
				}
				if a.exportAuthorityFingerprint != "" && fingerprint != a.exportAuthorityFingerprint {
					continue
				}

				// export certificates in the old 1.0 format where host and user
				// certificate authorities were exported in the known_hosts format.
				if a.compatVersion == "1.0" {
					castr, err := hostCAFormat(ca, keyBytes, client)
					if err != nil {
						return trace.Wrap(err)
					}

					fmt.Println(castr)
					continue
				}

				// export certificate authority in user or host ca format
				var castr string
				switch ca.GetType() {
				case services.UserCA:
					castr, err = userCAFormat(ca, keyBytes)
				case services.HostCA:
					castr, err = hostCAFormat(ca, keyBytes, client)
				default:
					return trace.BadParameter("unknown user type: %q", ca.GetType())
				}
				if err != nil {
					return trace.Wrap(err)
				}

				// print the export friendly string
				fmt.Println(castr)
			}
		}
	}
	return nil
}

// GenerateKeys generates a new keypair
func (a *AuthCommand) GenerateKeys() error {
	keygen := native.New()
	defer keygen.Close()
	privBytes, pubBytes, err := keygen.GenerateKeyPair("")
	if err != nil {
		return trace.Wrap(err)
	}
	err = ioutil.WriteFile(a.genPubPath, pubBytes, 0600)
	if err != nil {
		return trace.Wrap(err)
	}

	err = ioutil.WriteFile(a.genPrivPath, privBytes, 0600)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("wrote public key to: %v and private key to: %v\n", a.genPubPath, a.genPrivPath)
	return nil
}

// GenerateAndSignKeys generates a new keypair and signs it for role
func (a *AuthCommand) GenerateAndSignKeys(clusterApi *auth.TunClient) error {
	// parse compatibility parameter
	compatibility, err := utils.CheckCompatibilityFlag(a.compatibility)
	if err != nil {
		return trace.Wrap(err)
	}

	// generate a keypair:
	key, err := client.NewKey()
	if err != nil {
		return trace.Wrap(err)
	}

	// sign it and produce a cert:
	key.Cert, err = clusterApi.GenerateUserCert(key.Pub, a.genUser, a.genTTL, compatibility)
	if err != nil {
		return trace.Wrap(err)
	}

	// write the cert+private key to the output:
	err = client.MakeIdentityFile(a.genUser, a.output, key, a.outputFormat)
	if err != nil {
		return trace.Wrap(err)
	}
	if a.output != "" {
		fmt.Printf("\nThe certificate has been written to %s\n", a.output)
	}
	return nil
}

// userCAFormat returns the certificate authority public key exported as a single
// line that can be placed in ~/.ssh/authorized_keys file. The format adheres to the
// man sshd (8) authorized_keys format, a space-separated list of: options, keytype,
// base64-encoded key, comment.
// For example:
//
//    cert-authority AAA... type=user&clustername=cluster-a
//
// URL encoding is used to pass the CA type and cluster name into the comment field.
func userCAFormat(ca services.CertAuthority, keyBytes []byte) (string, error) {
	comment := url.Values{
		"type":        []string{string(services.UserCA)},
		"clustername": []string{ca.GetClusterName()},
	}

	return fmt.Sprintf("cert-authority %s %s", strings.TrimSpace(string(keyBytes)), comment.Encode()), nil
}

// hostCAFormat returns the certificate authority public key exported as a single line
// that can be placed in ~/.ssh/authorized_hosts. The format adheres to the man sshd (8)
// authorized_hosts format, a space-separated list of: marker, hosts, key, and comment.
// For example:
//
//    @cert-authority *.cluster-a ssh-rsa AAA... type=host
//
// URL encoding is used to pass the CA type and allowed logins into the comment field.
func hostCAFormat(ca services.CertAuthority, keyBytes []byte, client *auth.TunClient) (string, error) {
	comment := url.Values{
		"type": []string{string(ca.GetType())},
	}

	roles, err := services.FetchRoles(ca.GetRoles(), client, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}
	allowedLogins, _ := roles.CheckLoginDuration(defaults.MinCertDuration + time.Second)
	if len(allowedLogins) > 0 {
		comment["logins"] = allowedLogins
	}

	return fmt.Sprintf("@cert-authority *.%s %s %s",
		ca.GetClusterName(), strings.TrimSpace(string(keyBytes)), comment.Encode()), nil
}
