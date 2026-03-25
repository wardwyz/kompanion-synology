package stats

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/vanadium23/kompanion/pkg/postgres"
)

const KOReaderFile = "statistics.sqlite3"

// KOReaderPGStats implements ReadingStats interface
type KOReaderPGStats struct {
	pg *postgres.Postgres
}

func NewKOReaderPGStats(pg *postgres.Postgres) *KOReaderPGStats {
	return &KOReaderPGStats{pg: pg}
}

func (s *KOReaderPGStats) Write(ctx context.Context, r io.ReadCloser, deviceName string) error {
	// make by temp files
	tempFile, err := os.CreateTemp("", fmt.Sprintf("%s-", deviceName))
	if err != nil {
		return err
	}
	filepath := tempFile.Name()
	defer tempFile.Close()

	_, err = io.Copy(tempFile, r)
	if err != nil {
		return err
	}

	go SyncDatabases(filepath, s.pg, deviceName)
	return nil
}

type BookStats struct {
	TotalReadPages     int
	TotalReadTime      int // in seconds
	AverageTimePerPage int // in seconds
	TotalReadDays      int
}

func (s *KOReaderPGStats) GetBookStats(ctx context.Context, fileHash string) (*BookStats, error) {
	query := `
		WITH daily_reads AS (
			SELECT DISTINCT DATE(start_time) as read_date
			FROM stats_page_stat_data
			WHERE koreader_partial_md5 = $1
		)
		SELECT 
			COUNT(DISTINCT page) as total_read_pages,
			SUM(duration) as total_read_time,
			COUNT(DISTINCT DATE(start_time)) as total_read_days
		FROM stats_page_stat_data
		WHERE koreader_partial_md5 = $1
	`

	var stats BookStats
	err := s.pg.Pool.QueryRow(ctx, query, fileHash).Scan(
		&stats.TotalReadPages,
		&stats.TotalReadTime,
		&stats.TotalReadDays,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get book stats: %w", err)
	}

	if stats.TotalReadPages > 0 {
		stats.AverageTimePerPage = stats.TotalReadTime / stats.TotalReadPages
	}

	return &stats, nil
}

func (s *KOReaderPGStats) GetGeneralStats(ctx context.Context, from, to time.Time) (*GeneralStats, error) {
	var stats GeneralStats

	// Get per book statistics
	bookQuery := `
		SELECT 
			b.title,
			COUNT(DISTINCT kpsd.page) as total_read_pages,
			SUM(kpsd.duration) as total_read_time,
			COUNT(DISTINCT DATE(kpsd.start_time)) as total_read_days
		FROM stats_page_stat_data kpsd
		JOIN stats_book b ON b.koreader_partial_md5 = kpsd.koreader_partial_md5 AND b.auth_device_name = kpsd.auth_device_name
		WHERE kpsd.start_time BETWEEN $1 AND $2
		GROUP BY b.title, b.koreader_partial_md5
		HAVING COUNT(DISTINCT kpsd.page) > 0
	`

	rows, err := s.pg.Pool.Query(ctx, bookQuery, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get book stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var bookStat BookStatsWithTitle
		err := rows.Scan(
			&bookStat.Title,
			&bookStat.TotalReadPages,
			&bookStat.TotalReadTime,
			&bookStat.TotalReadDays,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan book stats: %w", err)
		}

		if bookStat.TotalReadPages > 0 {
			bookStat.AverageTimePerPage = int(bookStat.TotalReadTime / bookStat.TotalReadPages)
		}

		// Add to totals
		stats.TotalReadPages += bookStat.TotalReadPages
		stats.TotalReadTime += bookStat.TotalReadTime
		stats.BookStats = append(stats.BookStats, bookStat)
	}
	stats.TotalBooks = len(stats.BookStats)

	dailyQuery := `
		SELECT
			DATE(kpsd.start_time) as read_date,
			b.title,
			COUNT(DISTINCT kpsd.page) as total_read_pages,
			COALESCE(SUM(kpsd.duration), 0) as total_read_time
		FROM stats_page_stat_data kpsd
		JOIN stats_book b ON b.koreader_partial_md5 = kpsd.koreader_partial_md5 AND b.auth_device_name = kpsd.auth_device_name
		WHERE kpsd.start_time BETWEEN $1 AND $2
		GROUP BY DATE(kpsd.start_time), b.title, b.koreader_partial_md5
		HAVING COUNT(DISTINCT kpsd.page) > 0
		ORDER BY read_date DESC, total_read_time DESC, b.title
	`

	dailyDetailsFrom := getDailyDetailsFrom(from, to)
	dailyRows, err := s.pg.Pool.Query(ctx, dailyQuery, dailyDetailsFrom, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily stats details: %w", err)
	}
	defer dailyRows.Close()

	dailyByDate := map[string]*DailyReadStats{}
	var dayOrder []string

	for dailyRows.Next() {
		var (
			readDate       time.Time
			bookTitle      string
			totalReadPages int
			totalReadTime  int
		)
		err = dailyRows.Scan(&readDate, &bookTitle, &totalReadPages, &totalReadTime)
		if err != nil {
			return nil, fmt.Errorf("failed to scan daily stats details: %w", err)
		}

		dateKey := readDate.Format("2006-01-02")
		dayStats, exists := dailyByDate[dateKey]
		if !exists {
			dayStats = &DailyReadStats{Date: readDate}
			dailyByDate[dateKey] = dayStats
			dayOrder = append(dayOrder, dateKey)
		}

		dayStats.Books = append(dayStats.Books, DailyBookStats{
			Title:          bookTitle,
			TotalReadPages: totalReadPages,
			TotalReadTime:  totalReadTime,
		})
		dayStats.TotalReadPages += totalReadPages
		dayStats.TotalReadTime += totalReadTime
	}

	for _, key := range dayOrder {
		day := dailyByDate[key]
		day.TotalBooks = len(day.Books)
		stats.DailyStats = append(stats.DailyStats, *day)
	}

	// Calculate days between dates for averages
	days := to.Sub(from).Hours() / 24
	if days > 0 {
		stats.AveragePagePerDay = int(float64(stats.TotalReadPages)/days + 0.5)
		stats.AverageTimePerDay = int(float64(stats.TotalReadTime)/days + 0.5)
	}

	return &stats, nil
}

func getDailyDetailsFrom(from, to time.Time) time.Time {
	recentWeekStart := to.AddDate(0, 0, -6)
	recentWeekStart = time.Date(
		recentWeekStart.Year(),
		recentWeekStart.Month(),
		recentWeekStart.Day(),
		0,
		0,
		0,
		0,
		recentWeekStart.Location(),
	)

	if recentWeekStart.After(from) {
		return recentWeekStart
	}

	return from
}

type DailyStats struct {
	Date               time.Time
	PageCount          int
	AvgDurationPerPage float64
}

func (s *KOReaderPGStats) GetDailyStats(ctx context.Context, from, to time.Time) ([]DailyStats, error) {
	query := `
		WITH RECURSIVE dates AS (
			SELECT date_trunc('day', $1::timestamp)::date as date
			UNION ALL
			SELECT (date + interval '1 day')::date
			FROM dates
			WHERE date < date_trunc('day', $2::timestamp)::date
		)
		SELECT 
			d.date,
			COALESCE(COUNT(DISTINCT kpsd.page), 0) as pages_read,
			CASE 
				WHEN COUNT(DISTINCT kpsd.page) > 0 THEN COALESCE(AVG(kpsd.duration), 0)
				ELSE 0 
			END as avg_duration_per_page
		FROM dates d
		LEFT JOIN stats_page_stat_data kpsd 
			ON date_trunc('day', kpsd.start_time)::date = d.date
			AND kpsd.start_time BETWEEN $1 AND $2
		GROUP BY d.date
		ORDER BY d.date;
	`

	rows, err := s.pg.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily stats: %w", err)
	}
	defer rows.Close()

	var stats []DailyStats
	for rows.Next() {
		var stat DailyStats
		err := rows.Scan(&stat.Date, &stat.PageCount, &stat.AvgDurationPerPage)
		if err != nil {
			return nil, fmt.Errorf("failed to scan daily stats: %w", err)
		}
		stats = append(stats, stat)
	}

	return stats, nil
}
