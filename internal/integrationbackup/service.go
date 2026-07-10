package integrationbackup

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hoaxisr/awg-manager/internal/hydraroute"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

const (
	ComponentSingbox    = "singbox"
	ComponentHydraRoute = "hydraroute"

	archiveRoot    = "awgm-backup"
	manifestName   = archiveRoot + "/manifest.json"
	settingsPrefix = archiveRoot + "/settings"
	filesPrefix    = archiveRoot + "/files"
	backupType     = "awgm-integration-backup"
	formatVersion  = 1
)

type singboxRuntime interface {
	ConfigDir() string
	ValidateConfigDir(context.Context) error
	ValidateConfigPath(context.Context, string) error
	Control(context.Context, string) error
	IsRunning() (bool, int)
}

type hydraRuntime interface {
	ReadConfig() (*hydraroute.Config, error)
	ListRules() ([]hydraroute.HRRule, []string, error)
	Control(string) error
	GetStatus() hydraroute.Status
}

type Service struct {
	dataDir  string
	hydraDir string
	settings *storage.SettingsStore
	singbox  singboxRuntime
	hydra    hydraRuntime
}

func NewService(dataDir string, settings *storage.SettingsStore, singbox singboxRuntime, hydra hydraRuntime) *Service {
	return &Service{
		dataDir:  dataDir,
		hydraDir: "/opt/etc/HydraRoute",
		settings: settings,
		singbox:  singbox,
		hydra:    hydra,
	}
}

type Manifest struct {
	Type          string         `json:"type"`
	FormatVersion int            `json:"formatVersion"`
	Component     string         `json:"component"`
	CreatedAt     time.Time      `json:"createdAt"`
	Files         []ManifestFile `json:"files"`
	Settings      *SettingsMeta  `json:"settings,omitempty"`
}

type ManifestFile struct {
	SourcePath  string `json:"sourcePath"`
	ArchivePath string `json:"archivePath"`
	Mode        uint32 `json:"mode"`
	SHA256      string `json:"sha256"`
	Required    bool   `json:"required"`
}

type SettingsMeta struct {
	ArchivePath string `json:"archivePath"`
	SHA256      string `json:"sha256"`
}

type RestoreResponse struct {
	Component string           `json:"component"`
	DryRun    bool             `json:"dryRun"`
	Outcomes  []RestoreOutcome `json:"outcomes"`
	Warnings  []string         `json:"warnings,omitempty"`
}

type RestoreOutcome struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Error  string `json:"error,omitempty"`
}

type fileSpec struct {
	sourcePath string
	actualPath string
	required   bool
}

type filePayload struct {
	spec fileSpec
	data []byte
	mode fs.FileMode
}

type deletePayload struct {
	sourcePath string
	actualPath string
}

type restorePlan struct {
	Writes   []filePayload
	Deletes  []deletePayload
	Settings []byte
}

type fileSnapshot struct {
	path    string
	existed bool
	data    []byte
	mode    fs.FileMode
}

type singboxSettingsSnapshot struct {
	CreateNDMSProxyForSingbox bool                          `json:"createNDMSProxyForSingbox"`
	SingboxRouter             storage.SingboxRouterSettings `json:"singboxRouter"`
}

type hydraSettingsSnapshot struct {
	GeoFile storage.GeoFileSettings `json:"geoFile"`
}

func (s *Service) CreateBackup(ctx context.Context, component string) (string, []byte, error) {
	component = normalizeComponent(component)
	if component == "" {
		return "", nil, fmt.Errorf("неподдерживаемый компонент backup")
	}

	files, settingsRaw, err := s.collect(component)
	if err != nil {
		return "", nil, err
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	manifest := Manifest{
		Type:          backupType,
		FormatVersion: formatVersion,
		Component:     component,
		CreatedAt:     time.Now().UTC(),
		Files:         make([]ManifestFile, 0, len(files)),
	}

	for _, file := range files {
		select {
		case <-ctx.Done():
			_ = zw.Close()
			return "", nil, ctx.Err()
		default:
		}
		archivePath := archiveEntryForSource(file.spec.sourcePath)
		h := &zip.FileHeader{
			Name:   archivePath,
			Method: zip.Deflate,
		}
		h.SetMode(file.mode)
		w, err := zw.CreateHeader(h)
		if err != nil {
			_ = zw.Close()
			return "", nil, err
		}
		if _, err := w.Write(file.data); err != nil {
			_ = zw.Close()
			return "", nil, err
		}
		manifest.Files = append(manifest.Files, ManifestFile{
			SourcePath:  file.spec.sourcePath,
			ArchivePath: archivePath,
			Mode:        uint32(file.mode.Perm()),
			SHA256:      sha256Hex(file.data),
			Required:    file.spec.required,
		})
	}

	if settingsRaw != nil {
		settingsArchivePath := settingsEntryForComponent(component)
		h := &zip.FileHeader{Name: settingsArchivePath, Method: zip.Deflate}
		h.SetMode(0o644)
		w, err := zw.CreateHeader(h)
		if err != nil {
			_ = zw.Close()
			return "", nil, err
		}
		if _, err := w.Write(settingsRaw); err != nil {
			_ = zw.Close()
			return "", nil, err
		}
		manifest.Settings = &SettingsMeta{
			ArchivePath: settingsArchivePath,
			SHA256:      sha256Hex(settingsRaw),
		}
	}

	sort.Slice(manifest.Files, func(i, j int) bool {
		return manifest.Files[i].SourcePath < manifest.Files[j].SourcePath
	})

	manifestRaw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = zw.Close()
		return "", nil, err
	}
	mw, err := zw.Create(manifestName)
	if err != nil {
		_ = zw.Close()
		return "", nil, err
	}
	if _, err := mw.Write(manifestRaw); err != nil {
		_ = zw.Close()
		return "", nil, err
	}
	if err := zw.Close(); err != nil {
		return "", nil, err
	}

	filename := fmt.Sprintf("awgm-%s-backup-%s.zip", component, manifest.CreatedAt.Format("20060102T150405Z"))
	return filename, buf.Bytes(), nil
}

func (s *Service) Restore(ctx context.Context, component string, archive []byte, dryRun bool) (*RestoreResponse, error) {
	component = normalizeComponent(component)
	if component == "" {
		return nil, fmt.Errorf("неподдерживаемый компонент restore")
	}

	manifest, files, settingsRaw, err := parseArchive(component, archive)
	if err != nil {
		return nil, err
	}
	plan, err := s.buildRestorePlan(component, manifest, files, settingsRaw)
	if err != nil {
		return nil, err
	}

	resp := &RestoreResponse{
		Component: component,
		DryRun:    dryRun,
		Outcomes:  make([]RestoreOutcome, 0, len(plan.Writes)+len(plan.Deletes)+1),
	}
	writeOutcomeCount := len(plan.Writes)
	for _, file := range plan.Writes {
		resp.Outcomes = append(resp.Outcomes, RestoreOutcome{
			Path:   file.spec.sourcePath,
			Action: plannedAction(dryRun),
		})
	}
	for _, file := range plan.Deletes {
		resp.Outcomes = append(resp.Outcomes, RestoreOutcome{
			Path:   file.sourcePath,
			Action: plannedDeleteAction(dryRun),
		})
	}
	if plan.Settings != nil {
		resp.Outcomes = append(resp.Outcomes, RestoreOutcome{
			Path:   "settings:/" + component,
			Action: plannedAction(dryRun),
		})
	}
	if dryRun {
		return resp, nil
	}

	if err := s.validatePlannedRestore(ctx, component, plan); err != nil {
		return nil, err
	}

	fileSnaps, err := snapshotTargets(plan)
	if err != nil {
		return nil, err
	}
	prevSettings, err := cloneCurrentSettings(s.settings)
	if err != nil {
		return nil, err
	}

	restoreRunning, stopFn, startFn, err := s.lifecycle(component, ctx)
	if err != nil {
		return nil, err
	}
	if restoreRunning && stopFn != nil {
		if err := stopFn(); err != nil {
			return nil, err
		}
	}

	rollback := func(cause error) error {
		for _, out := range resp.Outcomes {
			if out.Action == "restored" || out.Action == "planned" {
				resp.Warnings = append(resp.Warnings, "выполнен rollback после ошибки восстановления")
				break
			}
		}
		for i := range resp.Outcomes {
			if resp.Outcomes[i].Action == "restored" || resp.Outcomes[i].Action == "deleted" {
				resp.Outcomes[i].Action = "rollback"
				resp.Outcomes[i].Error = cause.Error()
			}
		}
		_ = restoreSnapshots(fileSnaps)
		if prevSettings != nil {
			_ = s.settings.Save(prevSettings)
		}
		if restoreRunning && startFn != nil {
			_ = startFn()
		}
		return cause
	}

	for i, file := range plan.Writes {
		if err := writeFile(file.spec.actualPath, file.data, file.mode); err != nil {
			return nil, rollback(err)
		}
		resp.Outcomes[i].Action = "restored"
	}

	for i, file := range plan.Deletes {
		if err := os.Remove(file.actualPath); err != nil && !os.IsNotExist(err) {
			return nil, rollback(err)
		}
		resp.Outcomes[writeOutcomeCount+i].Action = "deleted"
	}

	if plan.Settings != nil {
		if err := s.applySettings(component, plan.Settings); err != nil {
			return nil, rollback(err)
		}
		resp.Outcomes[len(resp.Outcomes)-1].Action = "restored"
	}

	if err := s.validate(component, ctx); err != nil {
		return nil, rollback(err)
	}

	if restoreRunning && startFn != nil {
		if err := startFn(); err != nil {
			return nil, rollback(err)
		}
	} else if component == ComponentSingbox {
		resp.Warnings = append(resp.Warnings, "конфигурация восстановлена, sing-box до восстановления был остановлен")
	}
	if component == ComponentHydraRoute && restoreRunning && s.hydra != nil && !s.hydra.GetStatus().Running {
		return nil, rollback(fmt.Errorf("HydraRoute не запустился после восстановления"))
	}
	return resp, nil
}

func (s *Service) buildRestorePlan(component string, manifest *Manifest, files map[string][]byte, settingsRaw []byte) (*restorePlan, error) {
	plan := &restorePlan{
		Writes:   make([]filePayload, 0, len(manifest.Files)),
		Deletes:  []deletePayload{},
		Settings: settingsRaw,
	}
	manifestSources := make(map[string]struct{}, len(manifest.Files))
	for _, mf := range manifest.Files {
		actualPath, err := s.resolveSource(component, mf.SourcePath)
		if err != nil {
			return nil, err
		}
		data, ok := files[mf.ArchivePath]
		if !ok {
			return nil, fmt.Errorf("в архиве отсутствует файл: %s", mf.ArchivePath)
		}
		if sha256Hex(data) != strings.ToLower(mf.SHA256) {
			return nil, fmt.Errorf("checksum не совпал для %s", mf.SourcePath)
		}
		plan.Writes = append(plan.Writes, filePayload{
			spec: fileSpec{
				sourcePath: mf.SourcePath,
				actualPath: actualPath,
				required:   mf.Required,
			},
			data: data,
			mode: fs.FileMode(mf.Mode).Perm(),
		})
		manifestSources[mf.SourcePath] = struct{}{}
	}
	if component == ComponentHydraRoute {
		if _, ok := manifestSources["/opt/etc/HydraRoute/hrneo.conf"]; !ok {
			return nil, fmt.Errorf("в архиве отсутствует обязательный файл: /opt/etc/HydraRoute/hrneo.conf")
		}
	}
	managedFiles, err := s.listManagedFiles(component)
	if err != nil {
		return nil, err
	}
	for sourcePath, actualPath := range managedFiles {
		if _, ok := manifestSources[sourcePath]; ok {
			continue
		}
		plan.Deletes = append(plan.Deletes, deletePayload{
			sourcePath: sourcePath,
			actualPath: actualPath,
		})
	}
	sort.Slice(plan.Deletes, func(i, j int) bool {
		return plan.Deletes[i].sourcePath < plan.Deletes[j].sourcePath
	})
	return plan, nil
}

func (s *Service) collect(component string) ([]filePayload, []byte, error) {
	specs, err := s.specs(component)
	if err != nil {
		return nil, nil, err
	}
	out := make([]filePayload, 0, len(specs))
	for _, spec := range specs {
		data, err := os.ReadFile(spec.actualPath)
		if err != nil {
			if os.IsNotExist(err) {
				if spec.required {
					return nil, nil, fmt.Errorf("обязательный файл отсутствует: %s", spec.actualPath)
				}
				continue
			}
			return nil, nil, err
		}
		st, err := os.Stat(spec.actualPath)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, filePayload{spec: spec, data: data, mode: st.Mode().Perm()})
	}
	settingsRaw, err := s.marshalSettings(component)
	if err != nil {
		return nil, nil, err
	}
	return out, settingsRaw, nil
}

func (s *Service) specs(component string) ([]fileSpec, error) {
	switch component {
	case ComponentSingbox:
		if s.singbox == nil {
			return nil, fmt.Errorf("backup sing-box недоступен")
		}
		configDir := s.singbox.ConfigDir()
		var specs []fileSpec
		appendMatches := func(dir string, pattern string, sourceRoot string, required bool) error {
			matches, err := filepath.Glob(filepath.Join(dir, pattern))
			if err != nil {
				return err
			}
			sort.Strings(matches)
			for _, match := range matches {
				info, err := os.Stat(match)
				if err != nil || info.IsDir() {
					continue
				}
				rel, err := filepath.Rel(dir, match)
				if err != nil {
					return err
				}
				specs = append(specs, fileSpec{
					sourcePath: path.Join(sourceRoot, filepath.ToSlash(rel)),
					actualPath: match,
					required:   required,
				})
			}
			return nil
		}
		if err := appendMatches(configDir, "*.json", "/opt/etc/awg-manager/singbox/config.d", true); err != nil {
			return nil, err
		}
		if err := appendMatches(filepath.Join(configDir, "disabled"), "*.json", "/opt/etc/awg-manager/singbox/config.d/disabled", false); err != nil {
			return nil, err
		}
		if err := appendMatches(filepath.Join(configDir, "rule-sets", "inline"), "*.*", "/opt/etc/awg-manager/singbox/rule-sets/inline", false); err != nil {
			return nil, err
		}
		if err := appendMatches(filepath.Join(configDir, "rule-sets", "dat"), "*.*", "/opt/etc/awg-manager/singbox/rule-sets/dat", false); err != nil {
			return nil, err
		}
		specs = append(specs,
			fileSpec{sourcePath: "/opt/etc/awg-manager/subscriptions.json", actualPath: filepath.Join(s.dataDir, "subscriptions.json")},
			fileSpec{sourcePath: "/opt/etc/awg-manager/singbox_watchdog.json", actualPath: filepath.Join(s.dataDir, "singbox_watchdog.json")},
			fileSpec{sourcePath: "/opt/etc/awg-manager/deviceproxy.json", actualPath: filepath.Join(s.dataDir, "deviceproxy.json")},
			fileSpec{sourcePath: "/opt/etc/awg-manager/dns-routes.json", actualPath: filepath.Join(s.dataDir, "dns-routes.json")},
			fileSpec{sourcePath: "/opt/etc/awg-manager/static-routes.json", actualPath: filepath.Join(s.dataDir, "static-routes.json")},
			fileSpec{sourcePath: "/opt/etc/awg-manager/client-routes.json", actualPath: filepath.Join(s.dataDir, "client-routes.json")},
		)
		return specs, nil
	case ComponentHydraRoute:
		specs := []fileSpec{
			{sourcePath: "/opt/etc/HydraRoute/hrneo.conf", actualPath: filepath.Join(s.hydraDir, "hrneo.conf"), required: true},
			{sourcePath: "/opt/etc/HydraRoute/domain.conf", actualPath: filepath.Join(s.hydraDir, "domain.conf")},
			{sourcePath: "/opt/etc/HydraRoute/ip.list", actualPath: filepath.Join(s.hydraDir, "ip.list")},
			{sourcePath: "/opt/etc/HydraRoute/hrneo.conf-opkg", actualPath: filepath.Join(s.hydraDir, "hrneo.conf-opkg")},
			{sourcePath: "/opt/etc/awg-manager/dns-routes.json", actualPath: filepath.Join(s.dataDir, "dns-routes.json")},
			{sourcePath: "/opt/etc/awg-manager/static-routes.json", actualPath: filepath.Join(s.dataDir, "static-routes.json")},
			{sourcePath: "/opt/etc/awg-manager/client-routes.json", actualPath: filepath.Join(s.dataDir, "client-routes.json")},
			{sourcePath: "/opt/etc/awg-manager/hydraroute-geodata.json", actualPath: filepath.Join(s.dataDir, "hydraroute-geodata.json")},
		}
		return specs, nil
	default:
		return nil, fmt.Errorf("неподдерживаемый компонент")
	}
}

func (s *Service) listManagedFiles(component string) (map[string]string, error) {
	out := map[string]string{}
	switch component {
	case ComponentSingbox:
		configDir := s.singbox.ConfigDir()
		appendMatches := func(root string, pattern string, sourceRoot string, filter func(string) bool) error {
			matches, err := filepath.Glob(filepath.Join(root, pattern))
			if err != nil {
				return err
			}
			for _, match := range matches {
				info, err := os.Stat(match)
				if err != nil || info.IsDir() {
					continue
				}
				rel, err := filepath.Rel(root, match)
				if err != nil {
					return err
				}
				rel = filepath.ToSlash(rel)
				if filter != nil && !filter(rel) {
					continue
				}
				out[path.Join(sourceRoot, rel)] = match
			}
			return nil
		}
		if err := appendMatches(configDir, "*.json", "/opt/etc/awg-manager/singbox/config.d", nil); err != nil {
			return nil, err
		}
		if err := appendMatches(filepath.Join(configDir, "disabled"), "*.json", "/opt/etc/awg-manager/singbox/config.d/disabled", nil); err != nil {
			return nil, err
		}
		ruleSetFilter := func(rel string) bool { return allowedRuleSetFile(rel) }
		if err := appendMatches(filepath.Join(configDir, "rule-sets", "inline"), "*.*", "/opt/etc/awg-manager/singbox/rule-sets/inline", ruleSetFilter); err != nil {
			return nil, err
		}
		if err := appendMatches(filepath.Join(configDir, "rule-sets", "dat"), "*.*", "/opt/etc/awg-manager/singbox/rule-sets/dat", ruleSetFilter); err != nil {
			return nil, err
		}
	case ComponentHydraRoute:
		candidates := []fileSpec{
			{sourcePath: "/opt/etc/HydraRoute/domain.conf", actualPath: filepath.Join(s.hydraDir, "domain.conf")},
			{sourcePath: "/opt/etc/HydraRoute/ip.list", actualPath: filepath.Join(s.hydraDir, "ip.list")},
			{sourcePath: "/opt/etc/awg-manager/dns-routes.json", actualPath: filepath.Join(s.dataDir, "dns-routes.json")},
			{sourcePath: "/opt/etc/awg-manager/static-routes.json", actualPath: filepath.Join(s.dataDir, "static-routes.json")},
			{sourcePath: "/opt/etc/awg-manager/client-routes.json", actualPath: filepath.Join(s.dataDir, "client-routes.json")},
			{sourcePath: "/opt/etc/awg-manager/hydraroute-geodata.json", actualPath: filepath.Join(s.dataDir, "hydraroute-geodata.json")},
		}
		for _, spec := range candidates {
			if _, err := os.Stat(spec.actualPath); err == nil {
				out[spec.sourcePath] = spec.actualPath
			} else if err != nil && !os.IsNotExist(err) {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("неподдерживаемый компонент")
	}
	return out, nil
}

func (s *Service) resolveSource(component string, sourcePath string) (string, error) {
	sourcePath = path.Clean(sourcePath)
	switch component {
	case ComponentSingbox:
		configRoot := "/opt/etc/awg-manager/singbox/config.d"
		if rel, ok := trimPosixPrefix(sourcePath, configRoot); ok {
			if !strings.HasSuffix(rel, ".json") || strings.Contains(rel, "/") {
				return "", fmt.Errorf("путь не разрешен allowlist: %s", sourcePath)
			}
			return filepath.Join(s.singbox.ConfigDir(), filepath.FromSlash(rel)), nil
		}
		disabledRoot := "/opt/etc/awg-manager/singbox/config.d/disabled"
		if rel, ok := trimPosixPrefix(sourcePath, disabledRoot); ok {
			if !strings.HasSuffix(rel, ".json") || strings.Contains(rel, "/") {
				return "", fmt.Errorf("путь не разрешен allowlist: %s", sourcePath)
			}
			return filepath.Join(s.singbox.ConfigDir(), "disabled", filepath.FromSlash(rel)), nil
		}
		inlineRoot := "/opt/etc/awg-manager/singbox/rule-sets/inline"
		if rel, ok := trimPosixPrefix(sourcePath, inlineRoot); ok {
			if !allowedRuleSetFile(rel) {
				return "", fmt.Errorf("путь не разрешен allowlist: %s", sourcePath)
			}
			return filepath.Join(s.singbox.ConfigDir(), "rule-sets", "inline", filepath.FromSlash(rel)), nil
		}
		datRoot := "/opt/etc/awg-manager/singbox/rule-sets/dat"
		if rel, ok := trimPosixPrefix(sourcePath, datRoot); ok {
			if !allowedRuleSetFile(rel) {
				return "", fmt.Errorf("путь не разрешен allowlist: %s", sourcePath)
			}
			return filepath.Join(s.singbox.ConfigDir(), "rule-sets", "dat", filepath.FromSlash(rel)), nil
		}
		switch sourcePath {
		case "/opt/etc/awg-manager/subscriptions.json":
			return filepath.Join(s.dataDir, "subscriptions.json"), nil
		case "/opt/etc/awg-manager/singbox_watchdog.json":
			return filepath.Join(s.dataDir, "singbox_watchdog.json"), nil
		case "/opt/etc/awg-manager/deviceproxy.json":
			return filepath.Join(s.dataDir, "deviceproxy.json"), nil
		case "/opt/etc/awg-manager/dns-routes.json":
			return filepath.Join(s.dataDir, "dns-routes.json"), nil
		case "/opt/etc/awg-manager/static-routes.json":
			return filepath.Join(s.dataDir, "static-routes.json"), nil
		case "/opt/etc/awg-manager/client-routes.json":
			return filepath.Join(s.dataDir, "client-routes.json"), nil
		}
	case ComponentHydraRoute:
		switch sourcePath {
		case "/opt/etc/HydraRoute/hrneo.conf":
			return filepath.Join(s.hydraDir, "hrneo.conf"), nil
		case "/opt/etc/HydraRoute/domain.conf":
			return filepath.Join(s.hydraDir, "domain.conf"), nil
		case "/opt/etc/HydraRoute/ip.list":
			return filepath.Join(s.hydraDir, "ip.list"), nil
		case "/opt/etc/HydraRoute/hrneo.conf-opkg":
			return filepath.Join(s.hydraDir, "hrneo.conf-opkg"), nil
		case "/opt/etc/awg-manager/dns-routes.json":
			return filepath.Join(s.dataDir, "dns-routes.json"), nil
		case "/opt/etc/awg-manager/static-routes.json":
			return filepath.Join(s.dataDir, "static-routes.json"), nil
		case "/opt/etc/awg-manager/client-routes.json":
			return filepath.Join(s.dataDir, "client-routes.json"), nil
		case "/opt/etc/awg-manager/hydraroute-geodata.json":
			return filepath.Join(s.dataDir, "hydraroute-geodata.json"), nil
		}
	}
	return "", fmt.Errorf("путь не разрешен allowlist: %s", sourcePath)
}

func (s *Service) marshalSettings(component string) ([]byte, error) {
	cur, err := s.settings.Get()
	if err != nil {
		return nil, err
	}
	switch component {
	case ComponentSingbox:
		return json.MarshalIndent(singboxSettingsSnapshot{
			CreateNDMSProxyForSingbox: cur.CreateNDMSProxyForSingbox,
			SingboxRouter:             cur.SingboxRouter,
		}, "", "  ")
	case ComponentHydraRoute:
		return json.MarshalIndent(hydraSettingsSnapshot{
			GeoFile: cur.GeoFile,
		}, "", "  ")
	default:
		return nil, fmt.Errorf("неподдерживаемый компонент")
	}
}

func (s *Service) applySettings(component string, raw []byte) error {
	cur, err := cloneCurrentSettings(s.settings)
	if err != nil {
		return err
	}
	switch component {
	case ComponentSingbox:
		var snap singboxSettingsSnapshot
		if err := json.Unmarshal(raw, &snap); err != nil {
			return err
		}
		cur.CreateNDMSProxyForSingbox = snap.CreateNDMSProxyForSingbox
		cur.SingboxRouter = snap.SingboxRouter
	case ComponentHydraRoute:
		var snap hydraSettingsSnapshot
		if err := json.Unmarshal(raw, &snap); err != nil {
			return err
		}
		cur.GeoFile = snap.GeoFile
	default:
		return fmt.Errorf("неподдерживаемый компонент")
	}
	return s.settings.Save(cur)
}

func (s *Service) lifecycle(component string, ctx context.Context) (bool, func() error, func() error, error) {
	switch component {
	case ComponentSingbox:
		running, _ := s.singbox.IsRunning()
		return running,
			nil,
			func() error { return s.singbox.Control(ctx, "restart") },
			nil
	case ComponentHydraRoute:
		if s.hydra == nil {
			return false, nil, nil, fmt.Errorf("backup HydraRoute недоступен")
		}
		running := s.hydra.GetStatus().Running
		return running,
			func() error { return s.hydra.Control("stop") },
			func() error { return s.hydra.Control("start") },
			nil
	default:
		return false, nil, nil, fmt.Errorf("неподдерживаемый компонент")
	}
}

func (s *Service) validate(component string, ctx context.Context) error {
	switch component {
	case ComponentSingbox:
		return s.singbox.ValidateConfigDir(ctx)
	case ComponentHydraRoute:
		if _, err := s.hydra.ReadConfig(); err != nil {
			return err
		}
		_, _, err := s.hydra.ListRules()
		return err
	default:
		return fmt.Errorf("неподдерживаемый компонент")
	}
}

func (s *Service) validatePlannedRestore(ctx context.Context, component string, plan *restorePlan) error {
	switch component {
	case ComponentSingbox:
		return s.validateSingboxTemp(ctx, plan)
	case ComponentHydraRoute:
		return s.validateHydraTemp(plan)
	default:
		return fmt.Errorf("неподдерживаемый компонент")
	}
}

func (s *Service) validateSingboxTemp(ctx context.Context, plan *restorePlan) error {
	tempDir, err := os.MkdirTemp("", "awgm-singbox-restore-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	if err := copyDirContents(s.singbox.ConfigDir(), tempDir); err != nil {
		return err
	}
	for _, file := range plan.Writes {
		if !strings.HasPrefix(file.spec.actualPath, s.singbox.ConfigDir()+string(os.PathSeparator)) &&
			file.spec.actualPath != s.singbox.ConfigDir() {
			continue
		}
		target, err := remapIntoTempRoot(s.singbox.ConfigDir(), tempDir, file.spec.actualPath)
		if err != nil {
			return err
		}
		if err := writeFile(target, file.data, file.mode); err != nil {
			return err
		}
	}
	for _, file := range plan.Deletes {
		if !strings.HasPrefix(file.actualPath, s.singbox.ConfigDir()+string(os.PathSeparator)) &&
			file.actualPath != s.singbox.ConfigDir() {
			continue
		}
		target, err := remapIntoTempRoot(s.singbox.ConfigDir(), tempDir, file.actualPath)
		if err != nil {
			return err
		}
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return s.singbox.ValidateConfigPath(ctx, tempDir)
}

func (s *Service) validateHydraTemp(plan *restorePlan) error {
	for _, file := range plan.Writes {
		switch file.spec.sourcePath {
		case "/opt/etc/HydraRoute/hrneo.conf":
			if err := validateHydraConfigBytes(file.data); err != nil {
				return err
			}
		case "/opt/etc/awg-manager/dns-routes.json",
			"/opt/etc/awg-manager/static-routes.json",
			"/opt/etc/awg-manager/client-routes.json",
			"/opt/etc/awg-manager/hydraroute-geodata.json":
			if len(bytes.TrimSpace(file.data)) == 0 {
				continue
			}
			var raw any
			if err := json.Unmarshal(file.data, &raw); err != nil {
				return fmt.Errorf("некорректный JSON в %s: %w", file.spec.sourcePath, err)
			}
		}
	}
	return nil
}

func parseArchive(component string, payload []byte) (*Manifest, map[string][]byte, []byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, nil, nil, err
	}
	files := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		if err := validateArchiveName(f.Name); err != nil {
			return nil, nil, nil, err
		}
		rc, err := f.Open()
		if err != nil {
			return nil, nil, nil, err
		}
		data, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			return nil, nil, nil, readErr
		}
		files[f.Name] = data
	}
	manifestRaw, ok := files[manifestName]
	if !ok {
		return nil, nil, nil, fmt.Errorf("в архиве отсутствует manifest.json")
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return nil, nil, nil, err
	}
	if manifest.Type != backupType {
		return nil, nil, nil, fmt.Errorf("неподдерживаемый тип backup: %s", manifest.Type)
	}
	if manifest.FormatVersion != formatVersion {
		return nil, nil, nil, fmt.Errorf("неподдерживаемая версия формата backup: %d", manifest.FormatVersion)
	}
	if normalizeComponent(manifest.Component) != component {
		return nil, nil, nil, fmt.Errorf("backup предназначен для другого компонента: %s", manifest.Component)
	}
	var settingsRaw []byte
	if manifest.Settings != nil {
		settingsRaw = files[manifest.Settings.ArchivePath]
		if settingsRaw == nil {
			return nil, nil, nil, fmt.Errorf("в архиве отсутствует settings snapshot: %s", manifest.Settings.ArchivePath)
		}
		if sha256Hex(settingsRaw) != strings.ToLower(manifest.Settings.SHA256) {
			return nil, nil, nil, fmt.Errorf("checksum settings snapshot не совпал")
		}
	}
	return &manifest, files, settingsRaw, nil
}

func snapshotFiles(files []filePayload) ([]fileSnapshot, error) {
	out := make([]fileSnapshot, 0, len(files))
	for _, file := range files {
		st, err := os.Stat(file.spec.actualPath)
		if err != nil {
			if os.IsNotExist(err) {
				out = append(out, fileSnapshot{path: file.spec.actualPath, existed: false})
				continue
			}
			return nil, err
		}
		data, err := os.ReadFile(file.spec.actualPath)
		if err != nil {
			return nil, err
		}
		out = append(out, fileSnapshot{
			path:    file.spec.actualPath,
			existed: true,
			data:    data,
			mode:    st.Mode().Perm(),
		})
	}
	return out, nil
}

func snapshotTargets(plan *restorePlan) ([]fileSnapshot, error) {
	seen := make(map[string]struct{}, len(plan.Writes)+len(plan.Deletes))
	targets := make([]filePayload, 0, len(plan.Writes)+len(plan.Deletes))
	for _, file := range plan.Writes {
		if _, ok := seen[file.spec.actualPath]; ok {
			continue
		}
		seen[file.spec.actualPath] = struct{}{}
		targets = append(targets, file)
	}
	for _, file := range plan.Deletes {
		if _, ok := seen[file.actualPath]; ok {
			continue
		}
		seen[file.actualPath] = struct{}{}
		targets = append(targets, filePayload{spec: fileSpec{actualPath: file.actualPath}})
	}
	return snapshotFiles(targets)
}

func restoreSnapshots(snaps []fileSnapshot) error {
	for _, snap := range snaps {
		if !snap.existed {
			_ = os.Remove(snap.path)
			continue
		}
		if err := writeFile(snap.path, snap.data, snap.mode); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(target string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if mode == 0 {
		mode = 0o644
	}
	temp, err := os.CreateTemp(filepath.Dir(target), ".awgm-restore-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, target); err != nil {
		return err
	}
	if dir, err := os.Open(filepath.Dir(target)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

func cloneCurrentSettings(store *storage.SettingsStore) (*storage.Settings, error) {
	cur, err := store.Get()
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(cur)
	if err != nil {
		return nil, err
	}
	var cloned storage.Settings
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

func validateArchiveName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("в архиве найден entry с пустым именем")
	}
	if path.IsAbs(name) {
		return fmt.Errorf("в архиве запрещен абсолютный путь: %s", name)
	}
	clean := path.Clean(name)
	if clean != name {
		return fmt.Errorf("в архиве найден небезопасный путь: %s", name)
	}
	if !strings.HasPrefix(name, archiveRoot+"/") {
		return fmt.Errorf("в архиве найден entry вне корня backup: %s", name)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("в архиве обнаружен path traversal: %s", name)
	}
	return nil
}

func archiveEntryForSource(sourcePath string) string {
	return filesPrefix + sourcePath
}

func settingsEntryForComponent(component string) string {
	return settingsPrefix + "/" + component + ".json"
}

func trimPosixPrefix(full string, prefix string) (string, bool) {
	if !strings.HasPrefix(full, prefix+"/") {
		return "", false
	}
	return strings.TrimPrefix(full, prefix+"/"), true
}

func allowedRuleSetFile(rel string) bool {
	if strings.Contains(rel, "/") {
		return false
	}
	return strings.HasSuffix(rel, ".json") || strings.HasSuffix(rel, ".srs") || strings.HasSuffix(rel, ".meta.json")
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func normalizeComponent(component string) string {
	switch strings.ToLower(strings.TrimSpace(component)) {
	case ComponentSingbox:
		return ComponentSingbox
	case ComponentHydraRoute:
		return ComponentHydraRoute
	default:
		return ""
	}
}

func plannedAction(dryRun bool) string {
	if dryRun {
		return "planned"
	}
	return "restored"
}

func plannedDeleteAction(dryRun bool) string {
	if dryRun {
		return "planned_delete"
	}
	return "deleted"
}

func remapIntoTempRoot(sourceRoot string, tempRoot string, actualPath string) (string, error) {
	rel, err := filepath.Rel(sourceRoot, actualPath)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("путь %s вне ожидаемого корня %s", actualPath, sourceRoot)
	}
	return filepath.Join(tempRoot, rel), nil
}

func copyDirContents(src string, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(cur string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if cur == src {
			return nil
		}
		rel, err := filepath.Rel(src, cur)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(cur)
		if err != nil {
			return err
		}
		return writeFile(target, data, info.Mode().Perm())
	})
}

func validateHydraConfigBytes(data []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "ipsettimeout", "ipsetmaxelem":
			if _, err := strconv.Atoi(strings.TrimSpace(val)); err != nil {
				return fmt.Errorf("некорректное число в hrneo.conf для %s", strings.TrimSpace(key))
			}
		case "autostart", "clearipset", "cidr", "ipsetenabletimeout", "directrouteenabled", "globalrouting", "conntrackflush":
			if !isHydraBool(strings.TrimSpace(val)) {
				return fmt.Errorf("некорректное булево значение в hrneo.conf для %s", strings.TrimSpace(key))
			}
		}
	}
	return scanner.Err()
}

func isHydraBool(v string) bool {
	switch strings.ToLower(v) {
	case "1", "0", "true", "false", "yes", "no", "on", "off":
		return true
	default:
		return false
	}
}
