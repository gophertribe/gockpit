package grafana

import (
	"encoding/json"
	"fmt"
	"io"
)

type Dashboard struct {
	Id            *int     `json:"id"`
	Uid           *string  `json:"uid"`
	Title         string   `json:"title"`
	Tags          []string `json:"tags"`
	Timezone      string   `json:"timezone"`
	SchemaVersion int      `json:"schemaVersion"`
	Version       int      `json:"version"`
	Refresh       string   `json:"refresh"`
}

type DashboardInfo struct {
	Dashboard Dashboard `json:"dashboard"`
	Meta      struct {
		IsStarred bool   `json:"isStarred"`
		Url       string `json:"url"`
		FolderId  int    `json:"folderId"`
		FolderUid string `json:"folderUid"`
		Slug      string `json:"slug"`
	} `json:"meta"`
}

type CreateUpdateDashboard struct {
	Dashboard Dashboard `json:"dashboard"`
	FolderId  int       `json:"folderId"`
	FolderUid string    `json:"folderUid"`
	Message   string    `json:"message"`
	Overwrite bool      `json:"overwrite"`
}

type DashboardInput struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	PluginId string `json:"pluginId"`
	Value    string `json:"value"`
}

type DashboardImport struct {
	Dashboard json.RawMessage  `json:"dashboard"`
	Overwrite bool             `json:"overwrite"`
	Inputs    []DashboardInput `json:"inputs"`
	FolderId  int              `json:"folderId"`
}

type DashboardImportResponse struct {
	PluginId         string `json:"pluginId"`
	Title            string `json:"title"`
	Imported         bool   `json:"imported"`
	ImportedUri      string `json:"importedUri"`
	ImportedUrl      string `json:"importedUrl"`
	Slug             string `json:"slug"`
	DashboardId      int    `json:"dashboardId"`
	FolderId         int    `json:"folderId"`
	ImportedRevision int    `json:"importedRevision"`
	Revision         int    `json:"revision"`
	Description      string `json:"description"`
	Path             string `json:"path"`
	Removed          bool   `json:"removed"`
}

type DashboardStructure struct {
	Inputs []struct {
		Name        string `json:"name"`
		Label       string `json:"label"`
		Description string `json:"description"`
		Type        string `json:"type"`
		PluginId    string `json:"pluginId"`
		PluginName  string `json:"pluginName"`
	} `json:"__inputs"`
	Requires []struct {
		Type    string `json:"type"`
		Id      string `json:"id"`
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"__requires"`
	Annotations struct {
		List []struct {
			BuiltIn    int    `json:"builtIn"`
			Datasource string `json:"datasource"`
			Enable     bool   `json:"enable"`
			Hide       bool   `json:"hide"`
			IconColor  string `json:"iconColor"`
			Name       string `json:"name"`
			Type       string `json:"type"`
		} `json:"list"`
	} `json:"annotations"`
	Editable     bool          `json:"editable"`
	GnetId       interface{}   `json:"gnetId"`
	GraphTooltip int           `json:"graphTooltip"`
	Id           int           `json:"id"`
	Links        []interface{} `json:"links"`
	Panels       []struct {
		Collapsed  bool   `json:"collapsed,omitempty"`
		Datasource string `json:"datasource"`
		GridPos    struct {
			H int `json:"h"`
			W int `json:"w"`
			X int `json:"x"`
			Y int `json:"y"`
		} `json:"gridPos"`
		Id          int           `json:"id"`
		Panels      []interface{} `json:"panels,omitempty"`
		Title       string        `json:"title"`
		Type        string        `json:"type"`
		FieldConfig struct {
			Defaults struct {
				Custom struct {
				} `json:"custom"`
				Mappings []struct {
					From  string `json:"from"`
					Id    int    `json:"id"`
					Text  string `json:"text"`
					To    string `json:"to"`
					Type  int    `json:"type"`
					Value string `json:"value,omitempty"`
				} `json:"mappings,omitempty"`
				Thresholds struct {
					Mode  string `json:"mode"`
					Steps []struct {
						Color string `json:"color"`
						Value *int   `json:"value"`
					} `json:"steps"`
				} `json:"thresholds,omitempty"`
				Decimals    int    `json:"decimals,omitempty"`
				DisplayName string `json:"displayName,omitempty"`
				Unit        string `json:"unit,omitempty"`
			} `json:"defaults"`
			Overrides []struct {
				Matcher struct {
					Id      string `json:"id"`
					Options string `json:"options"`
				} `json:"matcher"`
				Properties []struct {
					Id    string      `json:"id"`
					Value interface{} `json:"value"`
				} `json:"properties"`
			} `json:"overrides"`
		} `json:"fieldConfig,omitempty"`
		Options struct {
			ColorMode     string `json:"colorMode,omitempty"`
			GraphMode     string `json:"graphMode,omitempty"`
			JustifyMode   string `json:"justifyMode,omitempty"`
			Orientation   string `json:"orientation,omitempty"`
			ReduceOptions struct {
				Calcs  []string `json:"calcs"`
				Fields string   `json:"fields"`
				Values bool     `json:"values"`
			} `json:"reduceOptions"`
			TextMode             string `json:"textMode,omitempty"`
			ShowThresholdLabels  bool   `json:"showThresholdLabels,omitempty"`
			ShowThresholdMarkers bool   `json:"showThresholdMarkers,omitempty"`
		} `json:"options,omitempty"`
		PluginVersion string `json:"pluginVersion,omitempty"`
		Targets       []struct {
			GroupBy []struct {
				Params []string `json:"params"`
				Type   string   `json:"type"`
			} `json:"groupBy"`
			OrderByTime  string `json:"orderByTime"`
			Policy       string `json:"policy"`
			Query        string `json:"query"`
			RefId        string `json:"refId"`
			ResultFormat string `json:"resultFormat"`
			Select       [][]struct {
				Params []string `json:"params"`
				Type   string   `json:"type"`
			} `json:"select"`
			Tags []interface{} `json:"tags"`
			Hide bool          `json:"hide,omitempty"`
		} `json:"targets,omitempty"`
		TimeFrom      *string           `json:"timeFrom,omitempty"`
		TimeShift     interface{}       `json:"timeShift"`
		Interval      string            `json:"interval,omitempty"`
		MaxDataPoints int               `json:"maxDataPoints,omitempty"`
		Description   string            `json:"description,omitempty"`
		AliasColors   map[string]string `json:"aliasColors,omitempty"`
		Bars          bool              `json:"bars,omitempty"`
		DashLength    int               `json:"dashLength,omitempty"`
		Dashes        bool              `json:"dashes,omitempty"`
		Fill          int               `json:"fill,omitempty"`
		FillGradient  int               `json:"fillGradient,omitempty"`
		HiddenSeries  bool              `json:"hiddenSeries,omitempty"`
		Legend        struct {
			Avg     bool `json:"avg"`
			Current bool `json:"current"`
			Max     bool `json:"max"`
			Min     bool `json:"min"`
			Show    bool `json:"show"`
			Total   bool `json:"total"`
			Values  bool `json:"values"`
		} `json:"legend,omitempty"`
		Lines           bool   `json:"lines,omitempty"`
		Linewidth       int    `json:"linewidth,omitempty"`
		NullPointMode   string `json:"nullPointMode,omitempty"`
		Percentage      bool   `json:"percentage,omitempty"`
		Pointradius     int    `json:"pointradius,omitempty"`
		Points          bool   `json:"points,omitempty"`
		Renderer        string `json:"renderer,omitempty"`
		SeriesOverrides []struct {
		} `json:"seriesOverrides,omitempty"`
		SpaceLength int           `json:"spaceLength,omitempty"`
		Stack       bool          `json:"stack,omitempty"`
		SteppedLine bool          `json:"steppedLine,omitempty"`
		Thresholds  []interface{} `json:"thresholds,omitempty"`
		TimeRegions []interface{} `json:"timeRegions,omitempty"`
		Tooltip     struct {
			Shared    bool   `json:"shared"`
			Sort      int    `json:"sort"`
			ValueType string `json:"value_type"`
		} `json:"tooltip,omitempty"`
		Xaxis struct {
			Buckets interface{}   `json:"buckets"`
			Mode    string        `json:"mode"`
			Name    interface{}   `json:"name"`
			Show    bool          `json:"show"`
			Values  []interface{} `json:"values"`
		} `json:"xaxis,omitempty"`
		Yaxes []struct {
			Format   string      `json:"format"`
			Label    *string     `json:"label"`
			LogBase  int         `json:"logBase"`
			Max      *string     `json:"max"`
			Min      *string     `json:"min"`
			Show     bool        `json:"show"`
			Decimals interface{} `json:"decimals"`
		} `json:"yaxes,omitempty"`
		Yaxis struct {
			Align      bool        `json:"align"`
			AlignLevel interface{} `json:"alignLevel"`
		} `json:"yaxis,omitempty"`
		HideTimeOverride bool `json:"hideTimeOverride,omitempty"`
	} `json:"panels"`
	Refresh       string        `json:"refresh"`
	SchemaVersion int           `json:"schemaVersion"`
	Style         string        `json:"style"`
	Tags          []interface{} `json:"tags"`
	Templating    struct {
		List []interface{} `json:"list"`
	} `json:"templating"`
	Time struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"time"`
	Timepicker struct {
		RefreshIntervals []string `json:"refresh_intervals"`
	} `json:"timepicker"`
	Timezone string `json:"timezone"`
	Title    string `json:"title"`
	Uid      string `json:"uid"`
	Version  int    `json:"version"`
}

func ParseStructure(r io.Reader) (*DashboardStructure, error) {
	var str DashboardStructure
	err := json.NewDecoder(r).Decode(&str)
	if err != nil {
		return nil, fmt.Errorf("could not parse structure: %w", err)
	}
	return &str, nil
}
