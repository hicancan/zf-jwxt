package jwglxt

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

const freeRoomModule = "N2155"

// FreeRooms lists the rooms of every campus that are free in one period of one
// day of one week, as the free-room page (查询空闲教室) answers.
func (c *Client) FreeRooms(ctx context.Context, t Term, week, weekday, period int) ([]map[string]any, error) {
	form := url.Values{
		"fwzt": {"cx"}, "xqh_id": {""}, "xnm": {t.xnm()}, "xqm": {t.xqm()},
		"cdlb_id": {""}, "cdejlb_id": {""}, "qszws": {""}, "jszws": {""}, "cdmc": {""}, "lh": {""},
		"jyfs": {"0"}, "cdjylx": {""},
		"zcd": {strconv.Itoa(1 << (week - 1))}, "xqj": {strconv.Itoa(weekday)}, "jcd": {strconv.Itoa(1 << (period - 1))},
		"_search": {"false"}, "nd": {now()},
		"queryModel.showCount": {"5000"}, "queryModel.currentPage": {"1"},
		"queryModel.sortName": {"cdbh"}, "queryModel.sortOrder": {"asc"}, "time": {"1"},
	}
	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := c.PostJSON(ctx, "/cdjy/cdjy_cxKxcdlb.html?doType=query&gnmkdm="+freeRoomModule, form, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// RoomsQuery queries /cdjy/cdjy_cxKxcdlb.html with arbitrary raw filters,
// preserving all 35 room fields.
func (c *Client) RoomsQuery(ctx context.Context, params url.Values, showCount, page int) (PagedResponse, error) {
	form := url.Values{"fwzt": {"cx"}, "jyfs": {"0"}}
	for k, vs := range params {
		form[k] = append([]string(nil), vs...)
	}
	return c.PagedQuery(ctx, "/cdjy/cdjy_cxKxcdlb.html?doType=query&gnmkdm="+freeRoomModule, form, showCount, page)
}

// RoomSchedule queries /cdjy/cdjy_cxAtyycdSchedule.html for a room's schedule.
func (c *Client) RoomSchedule(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.PostRaw(ctx, "/cdjy/cdjy_cxAtyycdSchedule.html?gnmkdm="+freeRoomModule, params)
}
