package signalform

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform/helper/schema"
)

const DASHBOARD_API_URL = "https://api.signalfx.com/v2/dashboard"

func dashboardResource() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"synced": &schema.Schema{
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Setting synced to 1 implies that the dashboard in SignalForm and SignalFx are identical",
			},
			"last_updated": &schema.Schema{
				Type:        schema.TypeFloat,
				Computed:    true,
				Description: "Latest timestamp the resource was updated",
			},
			"name": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the dashboard",
			},
			"description": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Description of the dashboard (Optional)",
			},
			"dashboard_group": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "The ID of the dashboard group that contains the dashboard. If an ID is not provided during creation, the dashboard will be placed in a newly created dashboard group",
			},
			"time_start": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Seconds since epoch to start displaying data",
			},
			"time_end": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Seconds since epoch to end displaying data",
			},
			"chart": &schema.Schema{
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "Chart ID and layout information for the charts in the dashboard",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"chart_id": &schema.Schema{
							Type:        schema.TypeString,
							Required:    true,
							Description: "ID of the chart to display",
						},
						"row": &schema.Schema{
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "The row to show the chart in (zero-based); if height > 1, this value represents the topmost row of the chart. (greater than or equal to 0)",
						},
						"column": &schema.Schema{
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "The column to show the chart in (zero-based); this value always represents the leftmost column of the chart. (between 0 and 11)",
						},
						"width": &schema.Schema{
							Type:        schema.TypeInt,
							Optional:    true,
							Default:     12,
							Description: "How many columns (out of a total of 12) the chart should take up. (between 1 and 12)",
						},
						"height": &schema.Schema{
							Type:        schema.TypeInt,
							Optional:    true,
							Default:     1,
							Description: "How many rows the chart should take up. (greater than or equal to 1)",
						},
					},
				},
			},
			"variable": &schema.Schema{
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "Dashboard variable to apply to each chart in the dashboard",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"property": &schema.Schema{
							Type:        schema.TypeString,
							Required:    true,
							Description: "A metric time series dimension or property name",
						},
						"alias": &schema.Schema{
							Type:        schema.TypeString,
							Required:    true,
							Description: "An alias for the dashboard variable. This text will appear as the label for the dropdown field on the dashboard",
						},
						"values": &schema.Schema{
							Type:        schema.TypeSet,
							Required:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "List of strings (which will be treated as an OR filter on the property)",
						},
					},
				},
			},
			"filter": &schema.Schema{
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "Filter to apply to each chart in the dashboard",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"property": &schema.Schema{
							Type:        schema.TypeString,
							Required:    true,
							Description: "A metric time series dimension or property name",
						},
						"negated": &schema.Schema{
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     false,
							Description: "(false by default) Whether this filter should be a \"not\" filter",
						},
						"values": &schema.Schema{
							Type:        schema.TypeSet,
							Required:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "List of strings (which will be treated as an OR filter on the property)",
						},
					},
				},
			},
		},

		Create: dashboardCreate,
		Read:   dashboardRead,
		Update: dashboardUpdate,
		Delete: dashboardDelete,
	}
}

/*
  Use Resource object to construct json payload in order to create a dashboard
*/
func getPayloadDashboard(d *schema.ResourceData) ([]byte, error) {
	payload := map[string]interface{}{
		"name":        d.Get("name").(string),
		"description": d.Get("description").(string),
		"groupId":     d.Get("dashboard_group").(string),
	}

	all_filters := make(map[string]interface{})
	if filters := getDashboardFilters(d); len(filters) > 0 {
		all_filters["sources"] = filters
	}
	if variables := getDashboardVariables(d); len(variables) > 0 {
		all_filters["variables"] = variables
	}
	if time := getDashboardTime(d); len(time) > 0 {
		all_filters["time"] = time
	}
	if len(all_filters) > 0 {
		payload["filters"] = all_filters
	}

	if charts := getDashboardCharts(d); len(charts) > 0 {
		payload["charts"] = charts
	}

	return json.Marshal(payload)
}

func getDashboardTime(d *schema.ResourceData) map[string]interface{} {
	time := make(map[string]interface{})
	if val, ok := d.GetOk("time_start"); ok {
		time["start"] = val.(int) * 1000
	}
	if val, ok := d.GetOk("time_end"); ok {
		time["end"] = val.(int) * 1000
	}
	if len(time) > 0 {
		return time
	}
	return nil
}

func getDashboardCharts(d *schema.ResourceData) []map[string]interface{} {
	charts := d.Get("chart").(*schema.Set).List()
	charts_list := make([]map[string]interface{}, len(charts))
	for i, chart := range charts {
		chart := chart.(map[string]interface{})
		item := make(map[string]interface{})

		item["chartId"] = chart["chart_id"].(string)
		item["row"] = chart["row"].(int)
		item["column"] = chart["column"].(int)
		item["height"] = chart["height"].(int)
		item["width"] = chart["width"].(int)

		charts_list[i] = item
	}
	return charts_list
}

func getDashboardVariables(d *schema.ResourceData) []map[string]interface{} {
	variables := d.Get("variable").(*schema.Set).List()
	vars_list := make([]map[string]interface{}, len(variables))
	for i, variable := range variables {
		variable := variable.(map[string]interface{})
		item := make(map[string]interface{})

		item["property"] = variable["property"].(string)
		item["alias"] = variable["alias"].(string)
		item["value"] = variable["values"].(*schema.Set).List()

		vars_list[i] = item
	}
	return vars_list
}

func getDashboardFilters(d *schema.ResourceData) []map[string]interface{} {
	filters := d.Get("filter").(*schema.Set).List()
	filter_list := make([]map[string]interface{}, len(filters))
	for i, filter := range filters {
		filter := filter.(map[string]interface{})
		item := make(map[string]interface{})

		item["property"] = filter["property"].(string)
		item["NOT"] = filter["negated"].(bool)
		item["value"] = filter["values"].(*schema.Set).List()

		filter_list[i] = item
	}
	return filter_list
}

func dashboardCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*signalformConfig)
	payload, err := getPayloadDashboard(d)
	if err != nil {
		return fmt.Errorf("Failed creating json payload: %s", err.Error())
	}

	return resourceCreate(DASHBOARD_API_URL, config.SfxToken, payload, d)
}

func dashboardRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*signalformConfig)
	url := fmt.Sprintf("%s/%s", DASHBOARD_API_URL, d.Id())

	return resourceRead(url, config.SfxToken, d)
}

func dashboardUpdate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*signalformConfig)
	payload, err := getPayloadDashboard(d)
	if err != nil {
		return fmt.Errorf("Failed creating json payload: %s", err.Error())
	}
	url := fmt.Sprintf("%s/%s", DASHBOARD_API_URL, d.Id())

	return resourceUpdate(url, config.SfxToken, payload, d)
}

func dashboardDelete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*signalformConfig)
	url := fmt.Sprintf("%s/%s", DASHBOARD_API_URL, d.Id())
	return resourceDelete(url, config.SfxToken, d)
}