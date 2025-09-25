package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/athom/hotel-merge/internal/domain"
	"github.com/athom/hotel-merge/internal/store"
)

// Service exposes the subset of the application service used by the API.
type Service interface {
	Query(ctx context.Context, destinations []int, hotelIDs []string) ([]domain.Hotel, error)
	ListRecords(ctx context.Context) ([]store.RecordSnapshot, error)
	GetRecord(ctx context.Context, destinationID int, hotelID string) (store.RecordSnapshot, bool, error)
}

// Handler bundles all HTTP handlers.
type Handler struct {
	svc Service
}

// NewHandler constructs a Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// Register attaches routes to the Gin engine.
func (h *Handler) Register(r *gin.Engine) {
	r.GET("/hotels", h.getHotels)
	r.GET("/debug/records", h.debugListRecords)
	r.GET("/debug/records/:destination/:hotel", h.debugShowRecord)
}

func (h *Handler) getHotels(c *gin.Context) {
	destinations, err := parseIntList(c, "destination", "destination_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hotelIDs := parseStringList(c, "hotels", "hotel_ids")

	if len(destinations) == 0 && len(hotelIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one filter (destination or hotels) is required"})
		return
	}

	results, err := h.svc.Query(c.Request.Context(), destinations, hotelIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range results {
		results[i].LastFetched = nil
		results[i].LastUpdated = time.Time{}
		results[i].LastMerged = time.Time{}
	}

	c.JSON(http.StatusOK, results)
}

func (h *Handler) debugListRecords(c *gin.Context) {
	records, err := h.svc.ListRecords(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "error: %v", err)
		return
	}

	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>Hotel Records</title><style>body{font-family:Arial,Helvetica,sans-serif;margin:24px;}table{border-collapse:collapse;width:100%;margin-top:16px;}th,td{border:1px solid #ccc;padding:8px;text-align:left;}th{background:#f5f5f5;}tr:nth-child(even){background:#fafafa;}a{text-decoration:none;color:#1976d2;}code{background:#f2f2f2;padding:2px 4px;border-radius:3px;}</style></head><body>")
	b.WriteString("<h1>Hotel Records</h1>")
	b.WriteString("<p>Total records: ")
	b.WriteString(fmt.Sprintf("%d", len(records)))
	b.WriteString("</p>")
	b.WriteString("<table><thead><tr><th>Destination</th><th>Hotel ID</th><th>Name</th><th>Needs Merge</th><th>Last Merged</th></tr></thead><tbody>")
	for _, rec := range records {
		name := ""
		if rec.Canonical != nil {
			name = rec.Canonical.Name
		}
		lastMerged := "-"
		if !rec.Meta.LastMerged.IsZero() {
			lastMerged = rec.Meta.LastMerged.Format(time.RFC3339)
		}
		url := fmt.Sprintf("/debug/records/%d/%s", rec.Key.DestinationID, urlPathEscape(rec.Key.HotelID))
		b.WriteString("<tr><td>")
		b.WriteString(fmt.Sprintf("%d", rec.Key.DestinationID))
		b.WriteString("</td><td><a href=\"")
		b.WriteString(html.EscapeString(url))
		b.WriteString("\">")
		b.WriteString(html.EscapeString(rec.Key.HotelID))
		b.WriteString("</a></td><td>")
		b.WriteString(html.EscapeString(name))
		b.WriteString("</td><td>")
		b.WriteString(fmt.Sprintf("%t", rec.NeedsMerge))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(lastMerged))
		b.WriteString("</td></tr>")
	}
	b.WriteString("</tbody></table>")
	b.WriteString("</body></html>")

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(b.String()))
}

func (h *Handler) debugShowRecord(c *gin.Context) {
	destID, err := strconv.Atoi(c.Param("destination"))
	if err != nil {
		c.String(http.StatusBadRequest, "invalid destination id")
		return
	}
	hotelID := c.Param("hotel")
	rec, ok, err := h.svc.GetRecord(c.Request.Context(), destID, hotelID)
	if err != nil {
		c.String(http.StatusInternalServerError, "error: %v", err)
		return
	}
	if !ok {
		c.String(http.StatusNotFound, "record not found")
		return
	}

	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>Record Detail</title><style>body{font-family:Arial,Helvetica,sans-serif;margin:24px;max-width:960px;}h1,h2,h3{margin-top:24px;}pre{background:#f6f8fa;padding:12px;border-radius:4px;overflow:auto;}table{border-collapse:collapse;margin-top:12px;}th,td{border:1px solid #ccc;padding:6px;text-align:left;}a{text-decoration:none;color:#1976d2;}code{background:#f2f2f2;padding:2px 4px;border-radius:3px;}</style></head><body>")
	b.WriteString("<a href=\"/debug/records\">&larr; Back to list</a>")
	b.WriteString("<h1>Destination ")
	b.WriteString(fmt.Sprintf("%d", rec.Key.DestinationID))
	b.WriteString(" / Hotel ")
	b.WriteString(html.EscapeString(rec.Key.HotelID))
	b.WriteString("</h1>")

	b.WriteString("<table><tbody>")
	b.WriteString(row("Needs Merge", fmt.Sprintf("%t", rec.NeedsMerge)))
	b.WriteString(row("Last Merged", formatTime(rec.Meta.LastMerged)))
	b.WriteString(row("Suppliers", fmt.Sprintf("%d", len(rec.Raw))))
	b.WriteString("</tbody></table>")

	b.WriteString("<h2>Canonical (Merged) Record</h2>")
	if rec.Canonical == nil {
		b.WriteString("<p><em>No merged record yet.</em></p>")
	} else {
		formatted := prettyJSON(rec.Canonical)
		b.WriteString("<pre>")
		b.WriteString(formatted)
		b.WriteString("</pre>")
	}

	keys := collectSupplierKeys(rec)
	for _, supplier := range keys {
		b.WriteString("<h3>Supplier ")
		b.WriteString(html.EscapeString(supplier))
		b.WriteString("</h3>")
		b.WriteString("<p>Last Fetched: ")
		b.WriteString(html.EscapeString(formatTime(rec.Meta.LastFetched[supplier])))
		b.WriteString("</p>")
		if normalized, ok := rec.Normalized[supplier]; ok {
			b.WriteString("<h4>Normalized View</h4><pre>")
			b.WriteString(prettyJSON(normalized))
			b.WriteString("</pre>")
		}
		if raw, ok := rec.Raw[supplier]; ok {
			b.WriteString("<h4>Raw Payload</h4><pre>")
			b.WriteString(prettyJSONBytes(raw))
			b.WriteString("</pre>")
		}
	}

	b.WriteString("</body></html>")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(b.String()))
}

func parseIntList(c *gin.Context, keys ...string) ([]int, error) {
	var raw []string
	for _, key := range keys {
		if values := c.QueryArray(key); len(values) > 0 {
			raw = append(raw, values...)
		}
		if value := c.Query(key); value != "" {
			raw = append(raw, strings.Split(value, ",")...)
		}
	}

	results := make([]int, 0, len(raw))
	seen := make(map[int]struct{})
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		v, err := strconv.Atoi(item)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		results = append(results, v)
	}
	return results, nil
}

func parseStringList(c *gin.Context, keys ...string) []string {
	var raw []string
	for _, key := range keys {
		if values := c.QueryArray(key); len(values) > 0 {
			raw = append(raw, values...)
		}
		if value := c.Query(key); value != "" {
			raw = append(raw, strings.Split(value, ",")...)
		}
	}

	results := make([]string, 0, len(raw))
	seen := make(map[string]struct{})
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		results = append(results, trimmed)
	}
	return results
}

func row(label, value string) string {
	return fmt.Sprintf("<tr><th>%s</th><td>%s</td></tr>", html.EscapeString(label), html.EscapeString(value))
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(time.RFC3339)
}

func collectSupplierKeys(rec store.RecordSnapshot) []string {
	set := make(map[string]struct{})
	for supplier := range rec.Raw {
		set[supplier] = struct{}{}
	}
	for supplier := range rec.Normalized {
		set[supplier] = struct{}{}
	}
	for supplier := range rec.Meta.LastFetched {
		set[supplier] = struct{}{}
	}
	keys := make([]string, 0, len(set))
	for supplier := range set {
		keys = append(keys, supplier)
	}
	sort.Strings(keys)
	return keys
}

func prettyJSON(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return html.EscapeString(fmt.Sprintf("%v", v))
	}
	return html.EscapeString(string(data))
}

func prettyJSONBytes(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var tmp interface{}
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return html.EscapeString(string(raw))
	}
	pretty, err := json.MarshalIndent(tmp, "", "  ")
	if err != nil {
		return html.EscapeString(string(raw))
	}
	return html.EscapeString(string(pretty))
}

func urlPathEscape(s string) string {
	return url.PathEscape(s)
}
