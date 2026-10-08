package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type AIReportConfig struct {
	APIKey        string
	BaseURL       string
	Model         string
	Timezone      string
	GenerateHour  int
	GenerateMiniute int
	PromptVersion string
}

type DailyAIReport struct {
	ReportDate  string                 `json:"reportDate"`
	Status      string                 `json:"status"`
	Content     string                 `json:"content"`
	Metrics     map[string]interface{} `json:"metrics"`
	Model       string                 `json:"model"`
	PromptVersion string               `json:"promptVersion"`
	Error       string                 `json:"error,omitempty"`
	GeneratedAt string                 `json:"generatedAt"`
	UpdatedAt   string                 `json:"updatedAt"`
}

func loadAIReportConfig() AIReportConfig {
	hour, _ := strconv.Atoi(envOrDefault("AI_REPORT_GENERATE_HOUR", "0"))
	minute, _ := strconv.Atoi(envOrDefault("AI_REPORT_GENERATE_MINUTE", "20"))
	return AIReportConfig{
		APIKey:        strings.TrimSpace(os.Getenv("AI_REPORT_API_KEY")),
		BaseURL:       strings.TrimSpace(os.Getenv("AI_REPORT_BASE_URL")),
		Model:         strings.TrimSpace(os.Getenv("AI_REPORT_MODEL")),
		Timezone:      envOrDefault("AI_REPORT_TIMEZONE", "Asia/Shanghai"),
		GenerateHour:  hour,
		GenerateMiniute: minute,
		PromptVersion: envOrDefault("AI_REPORT_PROMPT_VERSION", "2026-09-28"),
	}
}

func (c AIReportConfig) enabled() bool {
	return c.APIKey != "" && c.BaseURL != "" && c.Model != ""
}

func reportLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("CST", 8*60*60)
	}
	return location
}

func reportDay(location *time.Location, value time.Time) string {
	return value.In(location).Format("2006-01-02")
}

func previousReportDay(location *time.Location, now time.Time) string {
	return reportDay(location, now.AddDate(0, 0, -1))
}

func ensureAIReportTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS station_daily_ai_report (
			report_date DATE NOT NULL COMMENT 'Report date in Asia/Shanghai',
			status VARCHAR(16) NOT NULL COMMENT 'RUNNING/COMPLETED/FAILED',
			content MEDIUMTEXT NULL COMMENT 'AI generated daily report',
			metrics_json JSON NULL COMMENT 'Aggregated metrics sent to the model',
			model VARCHAR(100) NOT NULL DEFAULT '',
			prompt_version VARCHAR(30) NOT NULL DEFAULT '',
			error_message VARCHAR(1000) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (report_date)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci
	`)
	return err
}

func (a *API) dailyAIReport(w http.ResponseWriter, r *http.Request) {
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	if date == "" {
		date = previousReportDay(reportLocation("Asia/Shanghai"), time.Now())
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("date must be YYYY-MM-DD"))
		return
	}
	if err := ensureAIReportTable(r.Context(), a.store.db); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var reportDate, status, model, promptVersion, generatedAt, updatedAt, errorMessage sql.NullString
	var content sql.NullString
	var metricsNull sql.NullString
	err := a.store.db.QueryRowContext(r.Context(), `
		SELECT DATE_FORMAT(report_date,'%Y-%m-%d'), status, content, metrics_json,
		       model, prompt_version, error_message,
		       DATE_FORMAT(created_at,'%Y-%m-%dT%H:%i:%sZ'),
		       DATE_FORMAT(updated_at,'%Y-%m-%dT%H:%i:%sZ')
		  FROM station_daily_ai_report
		 WHERE report_date = ?
	`, date).Scan(&reportDate, &status, &content, &metricsNull, &model, &promptVersion, &errorMessage, &generatedAt, &updatedAt)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": a.cfg.AIReport.enabled(),
			"status": "MISSING",
			"reportDate": date,
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	metricsPayload := map[string]interface{}{}
	if metricsNull.Valid && strings.TrimSpace(metricsNull.String) != "" {
		_ = json.Unmarshal([]byte(metricsNull.String), &metricsPayload)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": a.cfg.AIReport.enabled(),
		"reportDate": reportDate.String,
		"status": status.String,
		"content": content.String,
		"metrics": metricsPayload,
		"model": model.String,
		"promptVersion": promptVersion.String,
		"error": errorMessage.String,
		"generatedAt": generatedAt.String,
		"updatedAt": updatedAt.String,
	})
}

type generateAIReportRequest struct {
	Date  string `json:"date"`
	Force bool   `json:"force"`
}

// aiReportDates 列出已保存的每日报告日期（倒序），供前端按日期查看历史报告。
func (a *API) aiReportDates(w http.ResponseWriter, r *http.Request) {
	if err := ensureAIReportTable(r.Context(), a.store.db); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	rows, err := a.store.db.QueryContext(r.Context(), `
		SELECT DATE_FORMAT(report_date,'%Y-%m-%d'), status
		  FROM station_daily_ai_report
		 ORDER BY report_date DESC
		 LIMIT 400`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	type dateItem struct {
		Date   string `json:"date"`
		Status string `json:"status"`
	}
	items := []dateItem{}
	for rows.Next() {
		var item dateItem
		if err := rows.Scan(&item.Date, &item.Status); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (a *API) generateDailyAIReport(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Admin-Key") != a.cfg.AdminKey {
		writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid admin key"))
		return
	}
	var body generateAIReportRequest
	if r.Body != nil {
		defer r.Body.Close()
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}
	}
	location, err := time.LoadLocation(a.cfg.AIReport.Timezone)
	if err != nil {
		location = time.FixedZone("CST", 8*60*60)
	}
	date := strings.TrimSpace(body.Date)
	if date == "" {
		date = previousReportDay(location, time.Now())
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("date must be YYYY-MM-DD"))
		return
	}

	report, err := a.runDailyAIReport(r.Context(), date, body.Force)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (a *API) runDailyAIReport(ctx context.Context, date string, force bool) (DailyAIReport, error) {
	if !a.cfg.AIReport.enabled() {
		return DailyAIReport{}, fmt.Errorf("AI report is disabled; set AI_REPORT_API_KEY, AI_REPORT_BASE_URL and AI_REPORT_MODEL")
	}
	if err := ensureAIReportTable(ctx, a.store.db); err != nil {
		return DailyAIReport{}, err
	}
	// Serialize generation to avoid duplicate model calls from several replicas.
	lockConn, err := a.store.db.Conn(ctx)
	if err != nil {
		return DailyAIReport{}, err
	}
	defer lockConn.Close()
	if err := lockConn.PingContext(ctx); err != nil {
		return DailyAIReport{}, err
	}
	if _, err := lockConn.ExecContext(ctx, "SELECT GET_LOCK('station_daily_ai_report', 0)"); err != nil {
		return DailyAIReport{}, err
	}
	defer func() {
		_, _ = lockConn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('station_daily_ai_report')")
	}()

	if !force {
		var status string
		err := a.store.db.QueryRowContext(ctx, "SELECT status FROM station_daily_ai_report WHERE report_date=?", date).Scan(&status)
		if err == nil && status == "COMPLETED" {
			return a.loadDailyAIReport(ctx, date)
		}
	} else if _, err := a.store.db.ExecContext(ctx, "DELETE FROM station_daily_ai_report WHERE report_date=?", date); err != nil {
		return DailyAIReport{}, err
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	_, _ = a.store.db.ExecContext(ctx, `
		INSERT INTO station_daily_ai_report
			(report_date,status,model,prompt_version,created_at,updated_at)
		VALUES (?,'RUNNING',?,?,?,?)
		ON DUPLICATE KEY UPDATE status=VALUES(status), model=VALUES(model),
			prompt_version=VALUES(prompt_version), updated_at=VALUES(updated_at)
	`, date, a.cfg.AIReport.Model, a.cfg.AIReport.PromptVersion, now, now)

	metrics, err := a.collectDailyReportMetrics(ctx, date)
	if err != nil {
		// Record the failure for /api/ai-report/daily, but return the original error:
		// markDailyAIReport returns nil on success, so returning its value would
		// turn the HTTP response into "200 + empty report" and swallow the reason.
		_ = a.markDailyAIReport(ctx, date, "FAILED", "", metrics, err)
		return DailyAIReport{}, err
	}
	content, err := callAIReportModel(a.cfg.AIReport, metrics)
	if err != nil {
		_ = a.markDailyAIReport(ctx, date, "FAILED", "", metrics, err)
		return DailyAIReport{}, err
	}
	if err := a.markDailyAIReport(ctx, date, "COMPLETED", content, metrics, nil); err != nil {
		return DailyAIReport{}, err
	}
	return a.loadDailyAIReport(ctx, date)
}

func (a *API) loadDailyAIReport(ctx context.Context, date string) (DailyAIReport, error) {
	var report DailyAIReport
	var content, metrics, errorText sql.NullString
	var generatedAt, updatedAt sql.NullTime
	err := a.store.db.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(report_date,'%Y-%m-%d'), status, content, metrics_json,
		       model, prompt_version, error_message, created_at, updated_at
		  FROM station_daily_ai_report WHERE report_date=?
	`, date).Scan(&report.ReportDate, &report.Status, &content, &metrics,
		&report.Model, &report.PromptVersion, &errorText, &generatedAt, &updatedAt)
	if err != nil {
		return report, err
	}
	report.Content = content.String
	report.Error = errorText.String
	if generatedAt.Valid { report.GeneratedAt = generatedAt.Time.Format("2006-01-02 15:04:05") }
	if updatedAt.Valid { report.UpdatedAt = updatedAt.Time.Format("2006-01-02 15:04:05") }
	report.Metrics = map[string]interface{}{}
	if metrics.Valid && strings.TrimSpace(metrics.String) != "" {
		_ = json.Unmarshal([]byte(metrics.String), &report.Metrics)
	}
	return report, nil
}

func (a *API) markDailyAIReport(ctx context.Context, date, status, content string, metrics map[string]interface{}, callErr error) error {
	metricsJSON, _ := json.Marshal(metrics)
	message := ""
	if callErr != nil { message = callErr.Error() }
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := a.store.db.ExecContext(ctx, `
		INSERT INTO station_daily_ai_report
			(report_date,status,content,metrics_json,model,prompt_version,error_message,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE status=VALUES(status), content=VALUES(content),
			metrics_json=VALUES(metrics_json), model=VALUES(model),
			prompt_version=VALUES(prompt_version), error_message=VALUES(error_message),
			updated_at=VALUES(updated_at)
	`, date, status, content, string(metricsJSON), a.cfg.AIReport.Model,
		a.cfg.AIReport.PromptVersion, message, now, now)
	return err
}

func (a *API) collectDailyReportMetrics(ctx context.Context, date string) (map[string]interface{}, error) {
	metrics := map[string]interface{}{"reportDate": date}
	var total, priced, withPiles, withTOU, flatOnly, pileTotal, pileIdle, pileBusy int
	var averagePrice sql.NullFloat64
	var updatedAt sql.NullInt64
	err := a.store.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       SUM(CASE WHEN TRIM(COALESCE(current_price,'')) <> '' THEN 1 ELSE 0 END),
		       SUM(CASE WHEN COALESCE(JSON_LENGTH(JSON_EXTRACT(result_payload,'$.chargingPiles')),0) > 0 THEN 1 ELSE 0 END),
		       SUM(CASE WHEN COALESCE(JSON_LENGTH(JSON_EXTRACT(result_payload,'$.fastPrices')),0)
		                   + COALESCE(JSON_LENGTH(JSON_EXTRACT(result_payload,'$.slowPrices')),0) > 1 THEN 1 ELSE 0 END),
		       SUM(CASE WHEN COALESCE(JSON_LENGTH(JSON_EXTRACT(result_payload,'$.fastPrices')),0)
		                   + COALESCE(JSON_LENGTH(JSON_EXTRACT(result_payload,'$.slowPrices')),0) = 1 THEN 1 ELSE 0 END),
		       COALESCE(SUM(COALESCE(fast_total,0)+COALESCE(super_total,0)+COALESCE(slow_total,0)),0),
		       COALESCE(SUM(COALESCE(fast_available,0)+COALESCE(super_available,0)+COALESCE(slow_available,0)),0),
		       AVG(CASE WHEN NULLIF(current_price,'') REGEXP '^[0-9]+([.][0-9]+)?$'
		                THEN CAST(NULLIF(current_price,'') AS DECIMAL(10,2)) END),
		       COALESCE(MAX(received_at),0)
		  FROM site_exploration_charging_station_result
	`).Scan(&total, &priced, &withPiles, &withTOU, &flatOnly, &pileTotal, &pileIdle, &averagePrice, &updatedAt)
	if err != nil { return metrics, err }
	pileBusy = maxInt(pileTotal-pileIdle, 0)
	idleRate, busyRate := 0.0, 0.0
	if pileIdle+pileBusy > 0 {
		idleRate = round(float64(pileIdle)*100/float64(pileIdle+pileBusy), 1)
		busyRate = round(float64(pileBusy)*100/float64(pileIdle+pileBusy), 1)
	}
	metrics["overall"] = map[string]interface{}{
		"stations": total, "pricedStations": priced, "stationsWithPileDetails": withPiles,
		"multiPeriodPriceStations": withTOU, "flatPriceStations": flatOnly,
		"pileTotal": pileTotal, "pileIdle": pileIdle, "pileBusy": pileBusy,
		"idleRate": idleRate, "busyRate": busyRate, "averagePrice": averagePrice.Float64,
	}

	var snapshotRows, snapshotStations, changedRows sql.NullInt64
	var avgIdle sql.NullFloat64
	err = a.store.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COUNT(DISTINCT source_key), SUM(is_changed),
		       AVG(CASE WHEN pile_idle + pile_busy > 0 THEN pile_idle * 100 / (pile_idle + pile_busy) END)
		  FROM site_exploration_charging_station_dynamic_history
		 WHERE FROM_UNIXTIME(captured_at) >= ? AND FROM_UNIXTIME(captured_at) < DATE_ADD(?, INTERVAL 1 DAY)
	`, date, date).Scan(&snapshotRows, &snapshotStations, &changedRows, &avgIdle)
	if err != nil { return metrics, err }
	metrics["dailySnapshots"] = map[string]interface{}{
		"rows": snapshotRows.Int64, "stations": snapshotStations.Int64,
		"changedRows": changedRows.Int64, "averageIdleRate": round(avgIdle.Float64, 1),
	}

	cityRows := []map[string]interface{}{}
	busyRows := []map[string]interface{}{}
	rows, err := a.store.db.QueryContext(ctx, `
		WITH latest AS (
			SELECT h.source_key, h.availability_json, h.price_json, h.pile_idle, h.pile_busy,
			       h.captured_at, h.id,
			       r.city, r.district, r.matched_station_name, r.current_price
			  FROM site_exploration_charging_station_dynamic_history h
			  LEFT JOIN site_exploration_charging_station_result r ON r.source_key=h.source_key
			 WHERE FROM_UNIXTIME(h.captured_at) >= ? AND FROM_UNIXTIME(h.captured_at) < DATE_ADD(?, INTERVAL 1 DAY)
		), ranked AS (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY source_key ORDER BY captured_at DESC, id DESC) rn
			  FROM latest
		)
		SELECT source_key, availability_json, city, district, matched_station_name, current_price,
		       pile_idle, pile_busy
		  FROM ranked WHERE rn=1
	`, date, date)
	if err != nil { return metrics, err }
	defer rows.Close()
	counts := map[string]int{"idle":0,"moderate":0,"full":0,"unknown":0}
	cities := map[string]map[string]interface{}{}
	for rows.Next() {
		var sourceKey, availabilityJSON, city, district, name, currentPrice sql.NullString
		var idle, busy int
		if err := rows.Scan(&sourceKey, &availabilityJSON, &city, &district, &name, &currentPrice, &idle, &busy); err != nil {
			return metrics, err
		}
		total := idle + busy
		status := "unknown"; rate := 0.0
		if total > 0 {
			rate = round(float64(idle)*100/float64(total),1)
			if idle >= total-idle && rate >= 50 { status = "idle" } else if rate >= 20 { status="moderate" } else { status="full" }
		}
		counts[status]++
		cityName := strings.TrimSpace(city.String); if cityName == "" { cityName = "未知地市" }
		cityStat, exists := cities[cityName]
		if !exists {
			cityStat = map[string]interface{}{"city":cityName,"stations":0,"pileIdle":0,"pileBusy":0}
			cities[cityName] = cityStat
		}
		cityStat["stations"] = cityStat["stations"].(int)+1
		cityStat["pileIdle"] = cityStat["pileIdle"].(int)+idle
		cityStat["pileBusy"] = cityStat["pileBusy"].(int)+busy
		if status=="full" && len(busyRows)<8 {
			busyRows = append(busyRows,map[string]interface{}{
				"station":name.String,"city":cityName,"district":district.String,
				"idle":idle,"busy":busy,"idleRate":rate,"price":currentPrice.String,
			})
		}
		_ = availabilityJSON
	}
	if err := rows.Err(); err != nil { return metrics, err }
	for _, item := range cities { cityRows = append(cityRows, item) }
	metrics["statusDistribution"] = counts
	metrics["cityDistribution"] = cityRows
	metrics["highLoadSample"] = busyRows
	return metrics, nil
}

func callAIReportModel(cfg AIReportConfig, metrics map[string]interface{}) (string, error) {
	metricsJSON, _ := json.Marshal(metrics)
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	endpoint := baseURL + "/chat/completions"
	if parsed, err := url.Parse(baseURL); err == nil && strings.HasSuffix(parsed.Path, "/chat/completions") {
		endpoint = baseURL
	}
	system := "你是重卡充电站运营分析助手。只使用用户提供的 JSON 数据，不要编造站点、数字或趋势。用简体中文输出简洁、专业的每日运行报告，使用 Markdown 小标题。"
	user := "请根据以下每日采集汇总生成运营日报，包含：核心结论、空闲率与负载分析、区域观察、数据质量提示、建议关注。数据 JSON：\n" + string(metricsJSON)
	payload := map[string]interface{}{
		"model": cfg.Model,
		"messages": []map[string]string{
			{"role":"system","content":system},
			{"role":"user","content":user},
		},
		"temperature": 0.35,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil { return "", err }
	req.Header.Set("Content-Type","application/json")
	req.Header.Set("Authorization","Bearer "+cfg.APIKey)
	client := &http.Client{Timeout: 90*time.Second}
	resp, err := client.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil { return "", err }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("model API HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw))[:min(len(strings.TrimSpace(string(raw))), 500)])
	}
	var result struct {
		Choices []struct{ Message struct{ Content string `json:"content"` } `json:"message"` } `json:"choices"`
		Error struct{ Message string `json:"message"` } `json:"error"`
	}
	if err := json.Unmarshal(raw,&result); err != nil { return "", err }
	if result.Error.Message != "" { return "", fmt.Errorf("%s", result.Error.Message) }
	if len(result.Choices)==0 { return "", fmt.Errorf("model returned no choices") }
	content := strings.TrimSpace(result.Choices[0].Message.Content)
	if content=="" { return "", fmt.Errorf("model returned empty content") }
	return content,nil
}

func StartAIReportScheduler(db *sql.DB,cfg AIReportConfig) {
	if !cfg.enabled() { return }
	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil { location = time.FixedZone("CST",8*60*60) }
	// cfg must be carried over: runDailyAIReport reads a.cfg.AIReport, so a bare
	// &API{store:...} would report "AI report is disabled" even when it is enabled.
	api := &API{store: &Store{db: db}, cfg: Config{AIReport: cfg}}
	go func() {
		for {
			now := time.Now().In(location)
			target := time.Date(now.Year(),now.Month(),now.Day(),cfg.GenerateHour,cfg.GenerateMiniute,0,0,location)
			if !now.Before(target) {
				yesterday := previousReportDay(location, now)
				var status string
				_ = db.QueryRow("SELECT status FROM station_daily_ai_report WHERE report_date=?",yesterday).Scan(&status)
				if status != "COMPLETED" {
					if _,err := api.runDailyAIReport(context.Background(),yesterday,false); err != nil {
						log.Printf("daily AI report %s failed: %v",yesterday,err)
					}
				}
			}
			next := time.Date(now.Year(),now.Month(),now.Day(),cfg.GenerateHour,cfg.GenerateMiniute,0,0,location)
			if !next.After(now) { next = next.AddDate(0,0,1) }
			time.Sleep(time.Until(next))
		}
	}()
}
