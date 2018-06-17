package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/elgatito/elementum/xbmc"

	"github.com/bogdanovich/dns_resolver"
	"github.com/dustin/go-humanize"
	"github.com/op/go-logging"
	"github.com/pbnjay/memory"
	"github.com/sanity-io/litter"
)

var log = logging.MustGetLogger("config")

const maxMemorySize = 200 * 1024 * 1024

// Configuration ...
type Configuration struct {
	DownloadPath              string
	TorrentsPath              string
	LibraryPath               string
	Info                      *xbmc.AddonInfo
	Platform                  *xbmc.Platform
	Language                  string
	TemporaryPath             string
	ProfilePath               string
	HomePath                  string
	XbmcPath                  string
	SpoofUserAgent            int
	KeepDownloading           int
	KeepFilesPlaying          int
	KeepFilesFinished         int
	DisableBgProgress         bool
	DisableBgProgressPlayback bool
	ForceUseTrakt             bool
	UseCacheSelection         bool
	UseCacheSearch            bool
	CacheSearchDuration       int
	ResultsPerPage            int
	EnableOverlayStatus       bool
	SilentStreamStart         bool
	ChooseStreamAuto          bool
	ForceLinkType             bool
	UseOriginalTitle          bool
	AddSpecials               bool
	ShowUnairedSeasons        bool
	ShowUnairedEpisodes       bool
	SmartEpisodeMatch         bool
	DownloadStorage           int
	AutoMemorySize            bool
	AutoMemorySizeStrategy    int
	MemorySize                int
	BufferSize                int
	UploadRateLimit           int
	DownloadRateLimit         int
	LimitAfterBuffering       bool
	ConnectionsLimit          int
	// SessionSave         int
	// ShareRatioLimit     int
	// SeedTimeRatioLimit  int
	SeedTimeLimit        int
	DisableUpload        bool
	DisableDHT           bool
	DisableTCP           bool
	DisableUTP           bool
	DisableUPNP          bool
	EncryptionPolicy     int
	ListenPortMin        int
	ListenPortMax        int
	ListenInterfaces     string
	ListenAutoDetectIP   bool
	ListenAutoDetectPort bool
	// OutgoingInterfaces string
	// TunedStorage        bool
	Scrobble bool

	TraktUsername        string
	TraktToken           string
	TraktRefreshToken    string
	TraktTokenExpiry     int
	TraktSyncFrequency   int
	TraktSyncCollections bool
	TraktSyncWatchlist   bool
	TraktSyncUserlists   bool
	TraktSyncWatched     bool
	TraktSyncWatchedBack bool

	UpdateFrequency int
	UpdateDelay     int
	UpdateAutoScan  bool
	PlayResume      bool
	UseCloudHole    bool
	CloudHoleKey    string
	TMDBApiKey      string

	OSDBUser         string
	OSDBPass         string
	OSDBLanguage     string
	OSDBAutoLanguage bool

	SortingModeMovies           int
	SortingModeShows            int
	ResolutionPreferenceMovies  int
	ResolutionPreferenceShows   int
	PercentageAdditionalSeeders int

	UsePublicDNS                 bool
	PublicDNSList                string
	OpennicDNSList               string
	CustomProviderTimeoutEnabled bool
	CustomProviderTimeout        int

	ProxyURL      string
	ProxyType     int
	ProxyEnabled  bool
	ProxyHost     string
	ProxyPort     int
	ProxyLogin    string
	ProxyPassword string

	CompletedMove       bool
	CompletedMoviesPath string
	CompletedShowsPath  string
}

// Addon ...
type Addon struct {
	ID      string
	Name    string
	Version string
	Enabled bool
}

var (
	config          = &Configuration{}
	lock            = sync.RWMutex{}
	settingsAreSet  = false
	settingsWarning = ""

	proxyTypes = []string{
		"Socks4",
		"Socks5",
		"HTTP",
		"HTTPS",
	}
)

var (
	// ResolverPublic ...
	ResolverPublic = dns_resolver.New([]string{"8.8.8.8", "8.8.4.4", "9.9.9.9"})

	// ResolverOpennic ...
	ResolverOpennic = dns_resolver.New([]string{"193.183.98.66", "172.104.136.243", "89.18.27.167"})
)

const (
	// ListenPort ...
	ListenPort = 65220
)

// Get ...
func Get() *Configuration {
	lock.RLock()
	defer lock.RUnlock()
	return config
}

// Reload ...
func Reload() *Configuration {
	log.Info("Reloading configuration...")

	defer func() {
		if r := recover(); r != nil {
			log.Warningf("Addon settings not properly set, opening settings window: %#v", r)

			message := "LOCALIZE[30314]"
			if settingsWarning != "" {
				message = settingsWarning
			}

			xbmc.AddonSettings("plugin.video.elementum")
			xbmc.Dialog("Elementum", message)

			waitForSettingsClosed()

			// Custom code to say python not to report this error
			os.Exit(5)
		}
	}()

	info := xbmc.GetAddonInfo()
	info.Path = xbmc.TranslatePath(info.Path)
	info.Profile = xbmc.TranslatePath(info.Profile)
	info.Home = xbmc.TranslatePath(info.Home)
	info.Xbmc = xbmc.TranslatePath(info.Xbmc)
	info.TempPath = filepath.Join(xbmc.TranslatePath("special://temp"), "elementum")

	platform := xbmc.GetPlatform()

	// If it's Windows and it's installed from Store - we should try to find real path
	// and change addon settings accordingly
	if platform != nil && strings.ToLower(platform.OS) == "windows" && strings.Contains(info.Xbmc, "XBMCFoundation") {
		path := findExistingPath([]string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "/Packages/XBMCFoundation.Kodi_4n2hpmxwrvr6p/LocalCache/Roaming/Kodi/"),
			filepath.Join(os.Getenv("APPDATA"), "/kodi/"),
		}, "/userdata/addon_data/"+info.ID)

		if path != "" {
			info.Path = strings.Replace(info.Path, info.Home, "", 1)
			info.Profile = strings.Replace(info.Profile, info.Home, "", 1)
			info.TempPath = strings.Replace(info.TempPath, info.Home, "", 1)
			info.Icon = strings.Replace(info.Icon, info.Home, "", 1)

			info.Path = filepath.Join(path, info.Path)
			info.Profile = filepath.Join(path, info.Profile)
			info.TempPath = filepath.Join(path, info.TempPath)
			info.Icon = filepath.Join(path, info.Icon)

			info.Home = path
		}
	}

	os.RemoveAll(info.TempPath)
	if err := os.MkdirAll(info.TempPath, 0777); err != nil {
		log.Infof("Could not create temporary directory: %#v", err)
	}

	if platform.OS == "android" {
		legacyPath := strings.Replace(info.Path, "/storage/emulated/0", "/storage/emulated/legacy", 1)
		if _, err := os.Stat(legacyPath); err == nil {
			info.Path = legacyPath
			info.Profile = strings.Replace(info.Profile, "/storage/emulated/0", "/storage/emulated/legacy", 1)
			log.Info("Using /storage/emulated/legacy path.")
		}
	}

	downloadPath := TranslatePath(xbmc.GetSettingString("download_path"))
	if downloadPath == "." {
		// xbmc.AddonSettings("plugin.video.elementum")
		// xbmc.Dialog("Elementum", "LOCALIZE[30113]")
		settingsWarning = "LOCALIZE[30113]"
		panic(settingsWarning)
	} else if err := IsWritablePath(downloadPath); err != nil {
		log.Errorf("Cannot write to location '%s': %#v", downloadPath, err)
		// xbmc.AddonSettings("plugin.video.elementum")
		// xbmc.Dialog("Elementum", err.Error())
		settingsWarning = err.Error()
		panic(settingsWarning)
	}
	log.Infof("Using download path: %s", downloadPath)

	libraryPath := TranslatePath(xbmc.GetSettingString("library_path"))
	if libraryPath == "." {
		libraryPath = downloadPath
	} else if err := IsWritablePath(libraryPath); err != nil {
		log.Error(err)
		// xbmc.Dialog("Elementum", err.Error())
		// xbmc.AddonSettings("plugin.video.elementum")
		settingsWarning = err.Error()
		panic(settingsWarning)
	}
	log.Infof("Using library path: %s", libraryPath)

	xbmcSettings := xbmc.GetAllSettings()
	settings := make(map[string]interface{})
	for _, setting := range xbmcSettings {
		switch setting.Type {
		case "enum":
			fallthrough
		case "number":
			value, _ := strconv.Atoi(setting.Value)
			settings[setting.Key] = value
		case "slider":
			var valueInt int
			var valueFloat float32
			switch setting.Option {
			case "percent":
				fallthrough
			case "int":
				floated, _ := strconv.ParseFloat(setting.Value, 32)
				valueInt = int(floated)
			case "float":
				floated, _ := strconv.ParseFloat(setting.Value, 32)
				valueFloat = float32(floated)
			}
			if valueFloat > 0 {
				settings[setting.Key] = valueFloat
			} else {
				settings[setting.Key] = valueInt
			}
		case "bool":
			settings[setting.Key] = (setting.Value == "true")
		default:
			settings[setting.Key] = setting.Value
		}
	}

	newConfig := Configuration{
		DownloadPath:              downloadPath,
		LibraryPath:               libraryPath,
		TorrentsPath:              filepath.Join(downloadPath, "Torrents"),
		Info:                      info,
		Platform:                  platform,
		Language:                  xbmc.GetLanguageISO639_1(),
		TemporaryPath:             info.TempPath,
		ProfilePath:               info.Profile,
		HomePath:                  info.Home,
		XbmcPath:                  info.Xbmc,
		DownloadStorage:           settings["download_storage"].(int),
		AutoMemorySize:            settings["auto_memory_size"].(bool),
		AutoMemorySizeStrategy:    settings["auto_memory_size_strategy"].(int),
		MemorySize:                settings["memory_size"].(int) * 1024 * 1024,
		BufferSize:                settings["buffer_size"].(int) * 1024 * 1024,
		UploadRateLimit:           settings["max_upload_rate"].(int) * 1024,
		DownloadRateLimit:         settings["max_download_rate"].(int) * 1024,
		SpoofUserAgent:            settings["spoof_user_agent"].(int),
		LimitAfterBuffering:       settings["limit_after_buffering"].(bool),
		KeepDownloading:           settings["keep_downloading"].(int),
		KeepFilesPlaying:          settings["keep_files_playing"].(int),
		KeepFilesFinished:         settings["keep_files_finished"].(int),
		DisableBgProgress:         settings["disable_bg_progress"].(bool),
		DisableBgProgressPlayback: settings["disable_bg_progress_playback"].(bool),
		ForceUseTrakt:             settings["force_use_trakt"].(bool),
		UseCacheSelection:         settings["use_cache_selection"].(bool),
		UseCacheSearch:            settings["use_cache_search"].(bool),
		CacheSearchDuration:       settings["cache_search_duration"].(int),
		ResultsPerPage:            settings["results_per_page"].(int),
		EnableOverlayStatus:       settings["enable_overlay_status"].(bool),
		SilentStreamStart:         settings["silent_stream_start"].(bool),
		ChooseStreamAuto:          settings["choose_stream_auto"].(bool),
		ForceLinkType:             settings["force_link_type"].(bool),
		UseOriginalTitle:          settings["use_original_title"].(bool),
		AddSpecials:               settings["add_specials"].(bool),
		ShowUnairedSeasons:        settings["unaired_seasons"].(bool),
		ShowUnairedEpisodes:       settings["unaired_episodes"].(bool),
		SmartEpisodeMatch:         settings["smart_episode_match"].(bool),
		// ShareRatioLimit:     settings["share_ratio_limit"].(int),
		// SeedTimeRatioLimit:  settings["seed_time_ratio_limit"].(int),
		SeedTimeLimit:        settings["seed_time_limit"].(int),
		DisableUpload:        settings["disable_upload"].(bool),
		DisableDHT:           settings["disable_dht"].(bool),
		DisableTCP:           settings["disable_tcp"].(bool),
		DisableUTP:           settings["disable_utp"].(bool),
		DisableUPNP:          settings["disable_upnp"].(bool),
		EncryptionPolicy:     settings["encryption_policy"].(int),
		ListenPortMin:        settings["listen_port_min"].(int),
		ListenPortMax:        settings["listen_port_max"].(int),
		ListenInterfaces:     settings["listen_interfaces"].(string),
		ListenAutoDetectIP:   settings["listen_autodetect_ip"].(bool),
		ListenAutoDetectPort: settings["listen_autodetect_port"].(bool),
		// OutgoingInterfaces: settings["outgoing_interfaces"].(string),
		// TunedStorage:        settings["tuned_storage"].(bool),
		ConnectionsLimit: settings["connections_limit"].(int),
		// SessionSave:         settings["session_save"].(int),
		Scrobble: settings["trakt_scrobble"].(bool),

		TraktUsername:        settings["trakt_username"].(string),
		TraktToken:           settings["trakt_token"].(string),
		TraktRefreshToken:    settings["trakt_refresh_token"].(string),
		TraktTokenExpiry:     settings["trakt_token_expiry"].(int),
		TraktSyncFrequency:   settings["trakt_sync"].(int),
		TraktSyncCollections: settings["trakt_sync_collections"].(bool),
		TraktSyncWatchlist:   settings["trakt_sync_watchlist"].(bool),
		TraktSyncUserlists:   settings["trakt_sync_userlists"].(bool),
		TraktSyncWatched:     settings["trakt_sync_watched"].(bool),
		TraktSyncWatchedBack: settings["trakt_sync_watchedback"].(bool),

		UpdateFrequency:  settings["library_update_frequency"].(int),
		UpdateDelay:      settings["library_update_delay"].(int),
		UpdateAutoScan:   settings["library_auto_scan"].(bool),
		PlayResume:       settings["play_resume"].(bool),
		UseCloudHole:     settings["use_cloudhole"].(bool),
		CloudHoleKey:     settings["cloudhole_key"].(string),
		TMDBApiKey:       settings["tmdb_api_key"].(string),
		OSDBUser:         settings["osdb_user"].(string),
		OSDBPass:         settings["osdb_pass"].(string),
		OSDBLanguage:     settings["osdb_language"].(string),
		OSDBAutoLanguage: settings["osdb_auto_language"].(bool),

		SortingModeMovies:           settings["sorting_mode_movies"].(int),
		SortingModeShows:            settings["sorting_mode_shows"].(int),
		ResolutionPreferenceMovies:  settings["resolution_preference_movies"].(int),
		ResolutionPreferenceShows:   settings["resolution_preference_shows"].(int),
		PercentageAdditionalSeeders: settings["percentage_additional_seeders"].(int),

		UsePublicDNS:                 settings["use_public_dns"].(bool),
		PublicDNSList:                settings["public_dns_list"].(string),
		OpennicDNSList:               settings["opennic_dns_list"].(string),
		CustomProviderTimeoutEnabled: settings["custom_provider_timeout_enabled"].(bool),
		CustomProviderTimeout:        settings["custom_provider_timeout"].(int),

		ProxyType:     settings["proxy_type"].(int),
		ProxyEnabled:  settings["proxy_enabled"].(bool),
		ProxyHost:     settings["proxy_host"].(string),
		ProxyPort:     settings["proxy_port"].(int),
		ProxyLogin:    settings["proxy_login"].(string),
		ProxyPassword: settings["proxy_password"].(string),

		CompletedMove:       settings["completed_move"].(bool),
		CompletedMoviesPath: settings["completed_movies_path"].(string),
		CompletedShowsPath:  settings["completed_shows_path"].(string),
	}

	// For memory storage we are changing configuration
	// 	to stop downloading after playback has stopped and so on
	if newConfig.DownloadStorage == 1 {
		newConfig.CompletedMove = false
		newConfig.KeepDownloading = 2
		newConfig.KeepFilesFinished = 2
		newConfig.KeepFilesPlaying = 2

		// TODO: Do we need this?
		// newConfig.SeedTimeLimit = 0

		// Calculate possible memory size, depending of selected strategy
		if newConfig.AutoMemorySize {
			if newConfig.AutoMemorySizeStrategy == 0 {
				newConfig.MemorySize = 40 * 1024 * 1024
			} else {
				pct := uint64(5 + (5 * (newConfig.AutoMemorySizeStrategy - 1)))
				mem := memory.TotalMemory() / 100 * pct
				if mem > 0 {
					newConfig.MemorySize = int(mem)
				}
				log.Debugf("Total system memory: %s\n", humanize.Bytes(memory.TotalMemory()))
				log.Debugf("Automatically selected memory size: %s\n", humanize.Bytes(uint64(newConfig.MemorySize)))
				if newConfig.MemorySize > maxMemorySize {
					log.Debugf("Selected memory size (%s) is bigger than maximum for auto-select (%s), so we decrease memory size to maximum allowed: %s", humanize.Bytes(uint64(mem)), humanize.Bytes(uint64(maxMemorySize)), humanize.Bytes(uint64(maxMemorySize)))
					newConfig.MemorySize = maxMemorySize
				}
			}
		}
	}

	// Set default Trakt Frequency
	if newConfig.TraktToken != "" && newConfig.TraktSyncFrequency == 0 {
		newConfig.TraktSyncFrequency = 6
	}

	// Setup OSDB language
	if newConfig.OSDBAutoLanguage || newConfig.OSDBLanguage == "" {
		newConfig.OSDBLanguage = newConfig.Language
	}

	// Collect proxy settings
	if newConfig.ProxyEnabled && newConfig.ProxyHost != "" {
		newConfig.ProxyURL = proxyTypes[newConfig.ProxyType] + "://"
		if newConfig.ProxyLogin != "" || newConfig.ProxyPassword != "" {
			newConfig.ProxyURL += newConfig.ProxyLogin + ":" + newConfig.ProxyPassword + "@"
		}

		newConfig.ProxyURL += newConfig.ProxyHost + ":" + strconv.Itoa(newConfig.ProxyPort)
	}

	// Reloading DNS resolvers
	newConfig.PublicDNSList = strings.Replace(newConfig.PublicDNSList, " ", "", -1)
	if newConfig.PublicDNSList != "" {
		ResolverPublic = dns_resolver.New(strings.Split(newConfig.PublicDNSList, ","))
	}

	newConfig.OpennicDNSList = strings.Replace(newConfig.OpennicDNSList, " ", "", -1)
	if newConfig.OpennicDNSList != "" {
		ResolverOpennic = dns_resolver.New(strings.Split(newConfig.OpennicDNSList, ","))
	}

	// Setting default connection limit per torrent.
	// This should be taken from gotorrent.Config{} defaults, but it's set internally.
	if newConfig.ConnectionsLimit == 0 {
		newConfig.ConnectionsLimit = 50
	}

	lock.Lock()
	config = &newConfig
	lock.Unlock()

	go CheckBurst()

	log.Debugf("Using configuration: %s", litter.Sdump(config))

	return config
}

// AddonIcon ...
func AddonIcon() string {
	return filepath.Join(Get().Info.Path, "icon.png")
}

// AddonResource ...
func AddonResource(args ...string) string {
	return filepath.Join(Get().Info.Path, "resources", filepath.Join(args...))
}

// TranslatePath ...
func TranslatePath(path string) string {
	// Do not translate nfs/smb path
	// if strings.HasPrefix(path, "nfs:") || strings.HasPrefix(path, "smb:") {
	// 	if !strings.HasSuffix(path, "/") {
	// 		path += "/"
	// 	}
	// 	return path
	// }
	return filepath.Dir(xbmc.TranslatePath(path))
}

// IsWritablePath ...
func IsWritablePath(path string) error {
	if path == "." {
		return errors.New("Path not set")
	}
	// TODO: Review this after test evidences come
	if strings.HasPrefix(path, "nfs") || strings.HasPrefix(path, "smb") {
		return fmt.Errorf("Network paths are not supported, change %s to a locally mounted path by the OS", path)
	}
	if p, err := os.Stat(path); err != nil || !p.IsDir() {
		if err != nil {
			return err
		}
		return fmt.Errorf("%s is not a valid directory", path)
	}
	writableFile := filepath.Join(path, ".writable")
	writable, err := os.Create(writableFile)
	if err != nil {
		return err
	}
	writable.Close()
	os.Remove(writableFile)
	return nil
}

func waitForSettingsClosed() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !xbmc.AddonSettingsOpened() {
				return
			}
		}
	}
}

// CheckBurst ...
func CheckBurst() {
	// Check for enabled providers and Elementum Burst
	hasBurst := false
	enabledProviders := make([]Addon, 0)
	for _, addon := range xbmc.GetAddons("xbmc.python.script", "executable", "all", []string{"name", "version", "enabled"}).Addons {
		if strings.HasPrefix(addon.ID, "script.elementum.") {
			if addon.Enabled == true {
				hasBurst = true
			}
			enabledProviders = append(enabledProviders, Addon{
				ID:      addon.ID,
				Name:    addon.Name,
				Version: addon.Version,
				Enabled: addon.Enabled,
			})
		}
	}
	if !hasBurst {
		log.Info("Updating Kodi add-on repositories for Burst...")
		xbmc.UpdateLocalAddons()
		xbmc.UpdateAddonRepos()
		time.Sleep(10 * time.Second)

		if xbmc.DialogConfirm("Elementum", "LOCALIZE[30271]") {
			xbmc.PlayURL("plugin://script.elementum.burst/")
			time.Sleep(4 * time.Second)
			for _, addon := range xbmc.GetAddons("xbmc.python.script", "executable", "all", []string{"name", "version", "enabled"}).Addons {
				if addon.ID == "script.elementum.burst" && addon.Enabled == true {
					hasBurst = true
				}
			}
			if hasBurst {
				for _, addon := range enabledProviders {
					xbmc.SetAddonEnabled(addon.ID, false)
				}
				xbmc.Notify("Elementum", "LOCALIZE[30272]", AddonIcon())
			} else {
				xbmc.Dialog("Elementum", "LOCALIZE[30273]")
			}
		}
	}
}

func findExistingPath(paths []string, addon string) string {
	// We add plugin folder to avoid getting dummy path, we should take care only for real folder
	for _, v := range paths {
		p := filepath.Join(v, addon)
		if _, err := os.Stat(p); err != nil {
			continue
		}

		return v
	}

	return ""
}
