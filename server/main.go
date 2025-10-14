package main

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/1password/onepassword-sdk-go"
	twerkle_merkle "github.com/ProjectApario/merkel/src/twerkle-merkle"
	gpgCrypto "github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/andreimerlescu/figtree/v2"
	sch "github.com/andreimerlescu/go-smartchan"
	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/didip/tollbooth_gin"
	"github.com/gin-gonic/gin"
	pkcs8 "github.com/youmark/pkcs8"
	"golang.org/x/crypto/ssh"

	"github.com/ProjectApario/merkel/tree"
)

const (
	defListener     = "127.0.0.1"
	defHTTP         = 8080
	defHTTPS        = 8443
	defDomain       = "app.here.local"
	defAccessLog    = "./merkel-server.access.log"
	defErrorLog     = "./merkel-server.error.log"
	defRateLimit    = 30
	defRateTTL      = 30
	defRateInterval = 30
	defServeCached  = false
	defCacheTTL     = "until-changed"
	defTreePath     = ""
)

var (
	ch_cert_reloader_cancel = sch.NewSmartChan(1)
	loadedCert              tls.Certificate
	mu_cert                 sync.RWMutex
	ctx                     context.Context
)

const (
	argTreePath                 string = "tree-path"
	argDomain                   string = "domain"
	argListener                 string = "listener"
	argHTTP                     string = "http"
	argHTTPS                    string = "https"
	argAccessLog                string = "access-log"
	argErrorLog                 string = "error-log"
	argRateLimit                string = "rate-limit"
	argRateInterval             string = "rate-interval"
	argRateTTL                  string = "rate-ttl"
	argSigningKey               string = "signing-key"
	argSigningKeyPass           string = "signing-key-pass"
	argServeCached              string = "serve-cached"
	argCacheTTL                 string = "cache-ttl"
	argCA                       string = "ca"
	argCert                     string = "cert"
	argKey                      string = "key"
	argKeyPass                  string = "key-pass"
	argSelfSigned               string = "self-signed"
	argSelfSignedReloadMinutes  string = "self-signed-reload-minutes"
	argSelfSignedExpiresInHours string = "self-signed-expires-in-hours"
	argSelfSignedSANIPList      string = "self-signed-san-ip-list"
	argSelfSignedDomains        string = "self-signed-domains"
	argSelfSignedCompany        string = "self-signed-company"
)

type Params struct {
	TreePath                 string
	Listener                 string
	HTTP                     int
	HTTPS                    int
	CA                       string
	Cert                     string
	Key                      string
	KeyPass                  string
	SelfSigned               bool
	SelfSignedReloadMinutes  int
	SelfSignedExpiresInHours int
	SelfSignedSANIPList      string
	SelfSignedDomains        string
	SelfSignedCompany        string
	Domain                   string
	AccessLog                string
	ErrorLog                 string
	RateLimit                int
	RateTTL                  int
	RateInterval             int
	ServeCached              bool
	CacheTTL                 string
	SigningKey               string
	SigningKeyPass           string
}

type Storage struct {
	B64Data   []string `json:"data"`
	Root      string   `json:"root,omitempty"`
	Signature string   `json:"signature,omitempty"`
}

var (
	params     *Params
	paramInfo  map[string]string
	mtree      *tree.MerkleTree
	mu         sync.RWMutex
	lastRoot   string
	opClient   *onepassword.Client
	opVaultIDs map[string]string // Cache vault IDs
	opMu       sync.RWMutex
)

func populateInfo() {
	paramInfo = map[string]string{
		argTreePath:                 "Path to Merkle Tree JSON file (default: \"./merkel.json\")",
		argListener:                 "Listener address (default: " + defListener + ")",
		argHTTP:                     "HTTP port (default: " + fmt.Sprintf("%d", defHTTP) + ")",
		argHTTPS:                    "HTTPS port (default: " + fmt.Sprintf("%d", defHTTPS) + ")",
		argCA:                       "CA cert path, op://vault/item/field, or raw PEM (default: \"\")",
		argCert:                     "Cert path, op://vault/item/field, or raw PEM (default: \"\")",
		argKey:                      "Key path, op://vault/item/field, or raw PEM (default: \"\")",
		argKeyPass:                  "Password for Key, op://vault/item/key-pass, or raw PEM (default: \"\")",
		argSelfSigned:               "Generate a self-signed SSL certificate for HTTPS",
		argSelfSignedReloadMinutes:  "Automatically re-load HTTPS certificate every n-Minutes",
		argSelfSignedExpiresInHours: "Automatically expire HTTPS certificate every n-Hours",
		argSelfSignedSANIPList:      "CSV List of IP Addresses for Self-Signed certificate",
		argSelfSignedDomains:        "CSV list of extra domains for Self-Signed certificate",
		argSelfSignedCompany:        "Company name to appear in Self-Signed certificate",
		argDomain:                   "Domain for redirects (default: " + defDomain + ")",
		argAccessLog:                "Access log path (default: " + defAccessLog + ")",
		argErrorLog:                 "Error log path (default: " + defErrorLog + ")",
		argRateLimit:                "Requests per interval (default: " + fmt.Sprintf("%d", defRateLimit) + ")",
		argRateInterval:             "Interval seconds (default: " + fmt.Sprintf("%d", defRateInterval) + ")",
		argRateTTL:                  "Rate Limiting TTL seconds (default: " + fmt.Sprintf("%d", defRateTTL) + ")",
		argServeCached:              "Serve cached tree (default: " + fmt.Sprint(defServeCached) + ")",
		argCacheTTL:                 "Cache TTL (default: " + defCacheTTL + ")",
		argSigningKey:               "Signing key path, op://vault/item/field, or GPG email (default: \"\")",
		argSigningKeyPass:           "Signing key password or op://vault/item/field (default: \"\")",
	}
}

func main() {
	populateInfo()
	ctx = context.Background()
	paramInfo = make(map[string]string)
	params = &Params{}
	opVaultIDs = make(map[string]string)
	figOpts := figtree.Options{
		Germinate:  true,
		ConfigFile: "config.yaml",
	}
	figs := figtree.With(figOpts)
	figs.NewString(argTreePath, defTreePath, paramInfo[argTreePath])
	figs.NewString(argListener, defListener, paramInfo[argListener])
	figs.NewInt(argHTTP, defHTTP, paramInfo[argHTTP])
	figs.NewInt(argHTTPS, defHTTPS, paramInfo[argHTTPS])
	figs.NewString(argCA, "", paramInfo[argCA])
	figs.NewString(argCert, "", paramInfo[argCert])
	figs.NewString(argKey, "", paramInfo[argKey])
	figs.NewString(argKeyPass, "", paramInfo[argKeyPass])
	figs.NewString(argDomain, defDomain, paramInfo[argDomain])
	figs.NewString(argAccessLog, defAccessLog, paramInfo[argAccessLog])
	figs.NewString(argErrorLog, defErrorLog, paramInfo[argErrorLog])
	figs.NewInt(argRateLimit, defRateLimit, paramInfo[argRateLimit])
	figs.NewInt(argRateInterval, defRateInterval, paramInfo[argRateInterval])
	figs.NewInt(argRateTTL, defRateTTL, paramInfo[argRateTTL])
	figs.NewBool(argServeCached, defServeCached, paramInfo[argServeCached])
	figs.NewString(argCacheTTL, defCacheTTL, paramInfo[argCacheTTL])
	figs.NewString(argSigningKey, "", paramInfo[argSigningKey])
	figs.NewString(argSigningKeyPass, "", paramInfo[argSigningKeyPass])
	figs.NewBool(argSelfSigned, false, paramInfo[argSelfSigned])
	figs.NewInt(argSelfSignedReloadMinutes, 3600, paramInfo[argSelfSignedReloadMinutes])
	figs.NewInt(argSelfSignedExpiresInHours, 168, paramInfo[argSelfSignedExpiresInHours])
	figs.NewString(argSelfSignedSANIPList, "", paramInfo[argSelfSignedSANIPList])
	figs.NewString(argSelfSignedDomains, "", paramInfo[argSelfSignedDomains])
	figs.NewString(argSelfSignedCompany, "", paramInfo[argSelfSignedCompany])
	err := figs.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	params = &Params{
		TreePath:                 *figs.String(argTreePath),
		Listener:                 *figs.String(argListener),
		HTTP:                     *figs.Int(argHTTP),
		HTTPS:                    *figs.Int(argHTTPS),
		CA:                       *figs.String(argCA),
		Cert:                     *figs.String(argCert),
		Key:                      *figs.String(argKey),
		KeyPass:                  *figs.String(argKeyPass),
		SelfSigned:               *figs.Bool(argSelfSigned),
		SelfSignedReloadMinutes:  *figs.Int(argSelfSignedReloadMinutes),
		SelfSignedExpiresInHours: *figs.Int(argSelfSignedExpiresInHours),
		SelfSignedSANIPList:      *figs.String(argSelfSignedSANIPList),
		SelfSignedDomains:        *figs.String(argSelfSignedDomains),
		SelfSignedCompany:        *figs.String(argSelfSignedCompany),
		Domain:                   *figs.String(argDomain),
		AccessLog:                *figs.String(argAccessLog),
		ErrorLog:                 *figs.String(argErrorLog),
		RateLimit:                *figs.Int(argRateLimit),
		RateTTL:                  *figs.Int(argRateTTL),
		RateInterval:             *figs.Int(argRateInterval),
		ServeCached:              *figs.Bool(argServeCached),
		CacheTTL:                 *figs.String(argCacheTTL),
		SigningKey:               *figs.String(argSigningKey),
		SigningKeyPass:           *figs.String(argSigningKeyPass),
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fmt.Println("Using Merkle-Twerkle To Print The Params")
	fmt.Println(twerkle_merkle.PrintStructTable(*params))

	// Initialize 1Password client
	opClient, err = onepassword.NewClient(
		context.Background(),
		onepassword.WithServiceAccountToken(os.Getenv("OP_SERVICE_ACCOUNT_TOKEN")),
		onepassword.WithIntegrationInfo("merkel-server", "1.0.0"),
	)
	if err != nil {
		log.Fatalf("Error initializing 1Password client: %v", err)
	}

	// Load tree from local merkel.json
	mtree, err = loadLocalTree(params.TreePath)
	if err != nil {
		log.Fatalf("Error loading tree: %v", err)
	}

	// Setup Gin
	//gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.Use(tollbooth_gin.LimitHandler(tollbooth.NewLimiter(float64(params.RateLimit), &limiter.ExpirableOptions{
		DefaultExpirationTTL: time.Duration(params.RateTTL) * time.Second,
		ExpireJobInterval:    time.Duration(params.RateInterval) * time.Second,
	})))

	// Endpoints
	r.GET("/merkel.json", serveMerkelJSON)
	r.GET("/merkel.pub", servePubKey)

	// Start HTTP Server
	go func(r *gin.Engine, params *Params) {
		server := &http.Server{
			Addr:    fmt.Sprintf("%s:%d", params.Listener, params.HTTP),
			Handler: r,
		}

		go func() {
			err := server.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("listen: %s\n", err)
			}
		}()

		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("Server Shutdown Failed: %+v", err)
		}

		log.Println("Server exiting properly")
	}(r, params)

	// Start HTTPS Server
	go func(r *gin.Engine, params *Params) {
		loadedCert = loadSSLCertificate(params)
		startCertReloader(params)
		server := &http.Server{
			Addr:    fmt.Sprintf("%s:%d", params.Listener, params.HTTPS),
			Handler: r,
			TLSConfig: &tls.Config{
				GetCertificate: getCertificate,
			},
		}

		go func() {
			err := server.ListenAndServeTLS("", "")
			if !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("ListenAndServeTLS(): %s", err)
			}
		}()

		<-ctx.Done()
		ctxShutDown, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctxShutDown); err != nil {
			log.Fatalf("Server forced to shutdown: %s", err)
		}

		log.Println("Server exiting properly")
	}(r, params)

	<-ctx.Done()
	fmt.Println("Thank you for using Merkel Server by Project Apario!")
}

// loadLocalTree loads the Merkle tree from a local JSON file.
func loadLocalTree(path string) (*tree.MerkleTree, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return tree.NewEmpty(), nil
	}

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %w", path, err)
	}

	if len(fileBytes) == 0 {
		return tree.NewEmpty(), nil
	}

	var s Storage
	if err := json.Unmarshal(fileBytes, &s); err != nil {
		return nil, fmt.Errorf("could not parse JSON from %s: %w", path, err)
	}

	var data [][]byte
	for _, b64 := range s.B64Data {
		item, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 data in file: %w", err)
		}
		data = append(data, item)
	}

	if len(data) == 0 {
		if s.Root != "" {
			return nil, errors.New("integrity check failed: empty data but non-empty root stored")
		}
		return tree.NewEmpty(), nil
	}

	mtree, err := tree.Build(data, true)
	if err != nil {
		return nil, err
	}

	if mtree.Root.Hash != s.Root {
		return nil, errors.New("integrity check failed: computed root does not match stored root")
	}

	return mtree, nil
}

func serveMerkelJSON(c *gin.Context) {
	mu.RLock()
	defer mu.RUnlock()

	// Helper to get root hash safely
	currentRoot := ""
	if mtree.Root != nil {
		currentRoot = mtree.Root.Hash
	}

	if params.ServeCached && lastRoot == currentRoot {
		// Serve cached (e.g., from a pre-marshaled var, but for now, just proceed to prepare)
	} else {
		// Rebuild/sign if changed
		signTree(mtree)
		lastRoot = currentRoot // Safe: "" if nil
	}

	storage := prepareStorage(mtree)
	jsonData, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		c.String(http.StatusInternalServerError, "Error marshaling JSON: %v", err)
		return
	}
	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, string(jsonData))
}

// servePubKey serves the public key for verification.
func servePubKey(c *gin.Context) {
	pubKey, err := getPublicKey(params.SigningKey)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching public key: %v", err)
		return
	}
	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, string(pubKey))
}

// loadTLSConfig loads TLS certificates and keys.
func loadTLSConfig() (cert, key string, err error) {
	caBytes, err := loadCertOrKey(params.CA)
	if err != nil {
		return "", "", fmt.Errorf("failed to load CA: %w", err)
	}
	certBytes, err := loadCertOrKey(params.Cert)
	if err != nil {
		return "", "", fmt.Errorf("failed to load cert: %w", err)
	}
	keyBytes, err := loadCertOrKey(params.Key)
	if err != nil {
		return "", "", fmt.Errorf("failed to load key: %w", err)
	}

	// Validate CA certificate
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caBytes) {
		return "", "", errors.New("failed to parse CA certificate")
	}

	// Validate server certificate and key
	_, err = tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return "", "", fmt.Errorf("failed to load TLS key pair: %w", err)
	}

	return string(certBytes), string(keyBytes), nil
}

// loadCertOrKey loads from file, op://, or raw PEM.
func loadCertOrKey(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("path is empty")
	}
	if strings.HasPrefix(path, "op://") {
		return fetchFrom1Password(context.Background(), path)
	}
	if _, err := os.Stat(path); err == nil {
		return os.ReadFile(path)
	}
	// Assume raw PEM if contains BEGIN
	if strings.Contains(path, "-----BEGIN ") {
		return []byte(path), nil
	}
	return nil, errors.New("invalid cert/key path or file not found")
}

// fetchFrom1Password retrieves a secret from 1Password using op://vault/item/field.
func fetchFrom1Password(ctx context.Context, ref string) ([]byte, error) {
	parts := strings.Split(strings.TrimPrefix(ref, "op://"), "/")
	if len(parts) != 3 {
		return nil, errors.New("invalid op:// format; expected op://vault/item/field")
	}
	vault, item, field := parts[0], parts[1], parts[2]

	// Cache vault ID
	opMu.RLock()
	vaultID, exists := opVaultIDs[vault]
	opMu.RUnlock()
	if !exists {
		var err error
		vaultID, err = vaultIdFor(ctx, vault)
		if err != nil {
			return nil, fmt.Errorf("failed to get vault ID for %s: %w", vault, err)
		}
		opMu.Lock()
		opVaultIDs[vault] = vaultID
		opMu.Unlock()
	}

	// Fetch item
	itemObj, err := opClient.Items().Get(ctx, vaultID, item)
	if err != nil {
		return nil, fmt.Errorf("failed to get item %s/%s: %w", vault, item, err)
	}

	// Find field
	for _, f := range itemObj.Fields {
		if f.Title == field || f.ID == field {
			return []byte(f.Value), nil
		}
	}
	return nil, fmt.Errorf("field %s not found in item %s/%s", field, vault, item)
}

// vaultIdFor retrieves the vault ID for a given vault name.
func vaultIdFor(ctx context.Context, name string) (string, error) {
	vaults, err := opClient.Vaults().List(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list vaults: %w", err)
	}
	for _, vault := range vaults {
		if strings.EqualFold(vault.Title, name) {
			return vault.ID, nil
		}
	}
	return "", fmt.Errorf("vault %s not found", name)
}

// keyRingFromKeyServer fetches a public PGP key from keys.openpgp.org by email address.
// Note that the key must have been explicitly verified by the owner to be discoverable
// via email.
func keyRingFromKeyServer(email string) (*gpgCrypto.Key, error) {
	keyserverURL := fmt.Sprintf("https://keys.openpgp.org/vks/v1/by-email/%s", url.QueryEscape(email))

	// Make the HTTP GET request.
	resp, err := http.Get(keyserverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request to keyserver: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("keyserver returned non-200 status code: %s", resp.Status)
	}

	// Read the response body.
	armoredKey, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read keyserver response body: %w", err)
	}

	// Parse the key data using GopenPGP.
	key, err := gpgCrypto.NewKeyFromArmored(string(armoredKey))
	if err != nil {
		return nil, fmt.Errorf("failed to parse armored key: %w", err)
	}

	return key, nil
}

// getPublicKey fetches GPG public key from MIT server or ED25519 public key.
func getPublicKey(keyRef string) ([]byte, error) {
	if strings.Contains(keyRef, "@") {
		// GPG email, fetch from MIT keyserver
		key, err := keyRingFromKeyServer(keyRef)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch GPG key for %s: %w", keyRef, err)
		}
		serialized, err := key.Serialize()
		if err != nil {
			return nil, fmt.Errorf("failed to serialize keyring: %w", err)
		}
		return serialized, nil
	}

	// ED25519 from file, op://, or raw PEM
	privPEM, err := loadCertOrKey(keyRef)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}
	block, _ := pem.Decode(privPEM)
	if block == nil {
		return nil, errors.New("invalid PEM format")
	}
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	edPriv, ok := priv.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not ED25519")
	}
	pub := edPriv.Public().(ed25519.PublicKey)
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to SSH public key: %w", err)
	}
	return ssh.MarshalAuthorizedKey(sshPub), nil
}

// signTree signs the storage with GPG or ED25519.
func signTree(mtree *tree.MerkleTree) {
	storage := prepareStorage(mtree)
	jsonBytes, err := json.Marshal(storage)
	if err != nil {
		log.Printf("Warning: failed to marshal storage for signing: %v", err)
		return
	}

	if strings.Contains(params.SigningKey, "@") {
		// GPG sign
		passphrase, err := loadCertOrKey(params.SigningKeyPass)
		if err != nil {
			log.Printf("Warning: failed to load GPG passphrase, signing without: %v", err)
			passphrase = nil
		}
		key, err := loadCertOrKey(params.SigningKey)
		if err != nil {
			log.Printf("Warning: failed to load GPG key, skipping signature: %v", err)
			return
		}
		signingKey, err := gpgCrypto.NewKeyFromArmored(string(key))
		if err != nil {
			log.Printf("Warning: failed to parse GPG key: %v", err)
			return
		}
		var keyring *gpgCrypto.KeyRing
		if passphrase != nil {
			signingKey, err = signingKey.Unlock(passphrase)
			if err != nil {
				log.Printf("Warning: failed to unlock GPG key: %v", err)
				return
			}
			keyring, err = gpgCrypto.NewKeyRing(signingKey)
			if err != nil {
				log.Printf("Warning: failed to create keyring: %v", err)
				return
			}
		} else {
			keyring, err = gpgCrypto.NewKeyRing(signingKey)
			if err != nil {
				log.Printf("Warning: failed to create keyring: %v", err)
				return
			}
		}
		message := gpgCrypto.NewPlainMessage(jsonBytes)
		signature, err := keyring.SignDetached(message)
		if err != nil {
			log.Printf("Warning: failed to sign with GPG: %v", err)
			return
		}
		storage.Signature = hex.EncodeToString(signature.GetBinary())
	} else {
		// ED25519 sign
		privPEM, err := loadCertOrKey(params.SigningKey)
		if err != nil {
			log.Printf("Warning: failed to load ED25519 key, skipping signature: %v", err)
			return
		}
		block, _ := pem.Decode(privPEM)
		if block == nil {
			log.Printf("Warning: invalid PEM format for ED25519 key")
			return
		}
		priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			log.Printf("Warning: failed to parse ED25519 key: %v", err)
			return
		}
		edPriv, ok := priv.(ed25519.PrivateKey)
		if !ok {
			log.Printf("Warning: private key is not ED25519")
			return
		}
		h := sha256.Sum256(jsonBytes)
		signature := ed25519.Sign(edPriv, h[:])
		storage.Signature = hex.EncodeToString(signature)
	}
}

// prepareStorage prepares Storage for JSON output.
func prepareStorage(mtree *tree.MerkleTree) *Storage {
	s := &Storage{
		B64Data: make([]string, len(mtree.Data())),
	}
	for i, item := range mtree.Data() {
		s.B64Data[i] = base64.StdEncoding.EncodeToString(item)
	}
	if mtree.Root != nil {
		s.Root = mtree.Root.Hash
	}
	return s
}

func loadSSLCertificate(params *Params) tls.Certificate {
	var cert tls.Certificate
	var err error

	// configure ssl
	if params.Cert != "" && params.Key != "" {
		cert, err = tls.LoadX509KeyPair(params.Cert, params.Key)
		if err != nil {
			if isPEMDecryptorNotFoundError(err) {
				decryptedKey, decryptErr := decryptPrivateKey(params.Key, params.KeyPass)
				if decryptErr != nil {
					log.Fatalf("Failed to decrypt private key: %v", decryptErr)
				}

				var pemBlock *pem.Block

				switch key := decryptedKey.(type) {
				case *rsa.PrivateKey:
					pemBlock = &pem.Block{
						Type:  "RSA PRIVATE KEY",
						Bytes: x509.MarshalPKCS1PrivateKey(key),
					}
				case *ecdsa.PrivateKey:
					ecPrivateKeyBytes, _ := x509.MarshalECPrivateKey(key)
					pemBlock = &pem.Block{
						Type:  "EC PRIVATE KEY",
						Bytes: ecPrivateKeyBytes,
					}
				case ed25519.PrivateKey:
					pemBlock = &pem.Block{
						Type:  "OPENSSH PRIVATE KEY",
						Bytes: key,
					}
				default:
					log.Fatalf("Unsupported key type: %T", decryptedKey)
				}

				privateKeyPem := pem.EncodeToMemory(pemBlock)

				var errKeyPair error
				cert, errKeyPair = tls.X509KeyPair([]byte(params.Cert), privateKeyPem)
				if errKeyPair != nil {
					log.Fatalf("Failed to load X509 key pair: %v", errKeyPair)
				}
			}
		}
	}

	// configure auto-tls
	if err != nil || params.SelfSigned {
		privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

		notBefore := time.Now()
		notAfter := notBefore.Add(time.Duration(params.SelfSignedExpiresInHours) * time.Hour)
		serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
		serialNumber, _ := rand.Int(rand.Reader, serialNumberLimit)

		var valid_ips []net.IP
		if strings.Contains(params.SelfSignedSANIPList, ",") {
			// assume CSV entry of IP addresses
			flag_ips := strings.ReplaceAll(params.SelfSignedSANIPList, " ", "")
			ips := strings.Split(flag_ips, ",")
			for _, ip := range ips {
				parsed_ip := net.ParseIP(ip)
				if parsed_ip == nil {
					log.Fatalf("failed to parse the ip address %v", params.SelfSignedSANIPList)
				}
				valid_ips = append(valid_ips, parsed_ip)
			}
		}

		var valid_domains []string
		valid_domains = append(valid_domains, params.Domain)
		if strings.Contains(params.SelfSignedDomains, ",") {
			flag_domains := strings.ReplaceAll(params.SelfSignedDomains, " ", "")
			domains := strings.Split(flag_domains, ",")
			for _, domain := range domains {
				_, err := url.Parse(domain)
				if err != nil {
					log.Printf("Invalid domain: %s\n", domain)
				} else {
					valid_domains = append(valid_domains, domain)
				}
			}
		}

		template := x509.Certificate{
			SerialNumber: serialNumber,
			Subject: pkix.Name{
				Organization: []string{params.SelfSignedCompany},
				CommonName:   params.Domain,
			},
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			IPAddresses:           valid_ips,
			DNSNames:              valid_domains,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			BasicConstraintsValid: true,
		}

		derBytes, _ := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)

		cert = tls.Certificate{
			Certificate: [][]byte{derBytes},
			PrivateKey:  privateKey,
		}
	}

	return cert
}

func startCertReloader(params *Params) {
	ticker := time.NewTicker(time.Duration(params.SelfSignedReloadMinutes) * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				newCert := loadSSLCertificate(params)
				mu_cert.Lock()
				loadedCert = newCert
				mu_cert.Unlock()
			case <-ch_cert_reloader_cancel.Chan():
				ticker.Stop()
				return
			}
		}
	}()
}

func getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	mu_cert.RLock()
	defer mu_cert.RUnlock()
	return &loadedCert, nil
}

func decryptPrivateKey(privateKeyPath, password string) (crypto.PrivateKey, error) {
	keyFile, err := os.Open(privateKeyPath)
	if err != nil {
		return nil, err
	}
	defer keyFile.Close()

	var keyBytes []byte
	buf := make([]byte, 1024)
	for {
		n, err := keyFile.Read(buf)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n == 0 {
			break
		}
		keyBytes = append(keyBytes, buf[:n]...)
	}

	pemBlock, _ := pem.Decode(keyBytes)
	if pemBlock == nil {
		return nil, errors.New("could not decode PEM block of private key")
	}

	if strings.Contains(string(keyBytes), "ENCRYPTED") {
		privKey, err := pkcs8.ParsePKCS8PrivateKey(pemBlock.Bytes, []byte(password))
		if err != nil {
			return nil, err
		}
		return privKey, nil
	} else {
		privKey, err := x509.ParsePKCS8PrivateKey(pemBlock.Bytes)
		if err != nil {
			return nil, err
		}
		return privKey, nil
	}
}

func isPEMDecryptorNotFoundError(err error) bool {
	return strings.Contains(err.Error(), "x509: decryption password incorrect")
}
