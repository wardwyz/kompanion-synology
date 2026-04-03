package metadata

import "testing"

func TestApplyDefaults(t *testing.T) {
	m := applyDefaults(Metadata{}, "uploads/三体.epub")
	if m.Title != "三体" {
		t.Fatalf("expected title 三体, got %q", m.Title)
	}
	if m.Author != "Unknown Author" {
		t.Fatalf("expected default author, got %q", m.Author)
	}
	if m.Description != "No description available" {
		t.Fatalf("expected default description, got %q", m.Description)
	}
	if m.Publisher != "" {
		t.Fatalf("expected empty publisher default, got %q", m.Publisher)
	}
}

func TestApplyDefaults_UsesFilenameTitleAuthorPattern(t *testing.T) {
	m := applyDefaults(Metadata{}, "uploads/三体 -- 刘慈欣.epub")
	if m.Title != "三体" {
		t.Fatalf("expected title 三体, got %q", m.Title)
	}
	if m.Author != "刘慈欣" {
		t.Fatalf("expected author 刘慈欣 from filename, got %q", m.Author)
	}
}

func TestApplyDefaults_UsesCompactPlusSeparator(t *testing.T) {
	m := applyDefaults(Metadata{}, "uploads/三体+刘慈欣.epub")
	if m.Title != "三体" {
		t.Fatalf("expected title 三体, got %q", m.Title)
	}
	if m.Author != "刘慈欣" {
		t.Fatalf("expected author 刘慈欣 from filename, got %q", m.Author)
	}
}

func TestMergeMetadataOverride(t *testing.T) {
	base := Metadata{
		Title:       "本地标题",
		Author:      "本地作者",
		Description: "本地描述",
		Publisher:   "本地出版社",
	}
	override := Metadata{
		Title:       "豆瓣标题",
		Author:      "豆瓣作者",
		Description: "豆瓣描述",
	}

	got := mergeMetadata(base, override)
	if got.Title != "豆瓣标题" {
		t.Fatalf("title was not overridden: %q", got.Title)
	}
	if got.Author != "豆瓣作者" {
		t.Fatalf("author was not overridden: %q", got.Author)
	}
	if got.Description != "豆瓣描述" {
		t.Fatalf("description was not overridden: %q", got.Description)
	}
	if got.Publisher != "本地出版社" {
		t.Fatalf("publisher should keep base value when override empty: %q", got.Publisher)
	}
}

func TestMergeScrapedMetadata_KeepsOriginalTitle(t *testing.T) {
	original := Metadata{
		Title:       "刀锋",
		Author:      "毛姆",
		Description: "本地描述",
	}
	scraped := Metadata{
		Title:       "刀锋(“故事高手”毛姆晚年重要作品，兰登书屋典藏本全文翻译)(果麦经典)",
		Author:      "威廉·萨默赛特·毛姆",
		Description: "豆瓣描述",
	}

	got := mergeScrapedMetadata(original, scraped)
	if got.Title != "刀锋" {
		t.Fatalf("expected original title to be preserved, got %q", got.Title)
	}
	if got.Author != "威廉·萨默赛特·毛姆" {
		t.Fatalf("expected author to still be enriched, got %q", got.Author)
	}
	if got.Description != "豆瓣描述" {
		t.Fatalf("expected description to still be enriched, got %q", got.Description)
	}
}

func TestMergeScrapedMetadata_UsesScrapedCleanerTitle(t *testing.T) {
	original := Metadata{
		Title: "刀锋(“故事高手”毛姆晚年重要作品，兰登书屋典藏本全文翻译)(果麦经典)",
	}
	scraped := Metadata{
		Title: "刀锋",
	}

	got := mergeScrapedMetadata(original, scraped)
	if got.Title != "刀锋" {
		t.Fatalf("expected scraped clean title to replace original, got %q", got.Title)
	}
}

func TestMergeScrapedMetadata_UsesScrapedTitleWhenOriginalMissing(t *testing.T) {
	original := Metadata{}
	scraped := Metadata{Title: "三体"}

	got := mergeScrapedMetadata(original, scraped)
	if got.Title != "三体" {
		t.Fatalf("expected scraped title when original is empty, got %q", got.Title)
	}
}

func TestShouldPreferScrapedTitle(t *testing.T) {
	tests := []struct {
		name     string
		original string
		scraped  string
		want     bool
	}{
		{
			name:     "decorated original title",
			original: "刀锋(“故事高手”毛姆晚年重要作品，兰登书屋典藏本全文翻译)(果麦经典)",
			scraped:  "刀锋",
			want:     true,
		},
		{
			name:     "same title",
			original: "刀锋",
			scraped:  "刀锋",
			want:     false,
		},
		{
			name:     "different unrelated title",
			original: "刀锋",
			scraped:  "月亮与六便士",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldPreferScrapedTitle(tt.original, tt.scraped)
			if got != tt.want {
				t.Fatalf("shouldPreferScrapedTitle(%q, %q) = %v, want %v", tt.original, tt.scraped, got, tt.want)
			}
		})
	}
}

func TestParseDoubanBookPageExtractsAuthorAndSeries(t *testing.T) {
	body := `
<html>
  <head>
	<meta property="og:title" content="三体"/>
	<meta property="og:description" content="科幻小说"/>
	<strong class="ll rating_num" property="v:average">9.2</strong>
  </head>
  <body>
	<div id="info">
	  <span class="pl">作者:</span> <a href="/author/1">刘慈欣</a><br/>
	  <span class="pl">丛书:</span> <a href="/series/1">地球往事三部曲</a><br/>
	  <span class="pl">出版社:</span> 重庆出版社<br/>
	  <span class="pl">出版年:</span> 2008-1<br/>
	  <span class="pl">ISBN:</span> 978-7-5366-9293-0<br/>
	</div>
  </body>
</html>`

	got := parseDoubanBookPage(body)
	if got.Author != "刘慈欣" {
		t.Fatalf("expected author 刘慈欣, got %q", got.Author)
	}
	if got.Series != "地球往事三部曲" {
		t.Fatalf("expected series 地球往事三部曲, got %q", got.Series)
	}
	if got.Publisher != "重庆出版社" {
		t.Fatalf("expected publisher 重庆出版社, got %q", got.Publisher)
	}
	if got.Date != "2008" {
		t.Fatalf("expected normalized publication year 2008, got %q", got.Date)
	}
	if got.ISBN != "9787536692930" {
		t.Fatalf("expected normalized ISBN, got %q", got.ISBN)
	}
	if got.SeriesIndex != "9.2" {
		t.Fatalf("expected douban rating in series index field, got %q", got.SeriesIndex)
	}
}

func TestParseDoubanBookPageExtractsPlainTextAuthor(t *testing.T) {
	body := `
<html>
  <body>
    <div id="info">
      <span class="pl">作者</span>: [日] 东野圭吾<br/>
    </div>
  </body>
</html>`

	got := parseDoubanBookPage(body)
	if got.Author != "[日] 东野圭吾" {
		t.Fatalf("expected plain text author, got %q", got.Author)
	}
}

func TestParseDoubanBookPageExtractsRatingWithDifferentClassOrder(t *testing.T) {
	body := `
<html>
  <head>
	<strong class="rating_num ll" property="v:average">8.7</strong>
  </head>
</html>`

	got := parseDoubanBookPage(body)
	if got.SeriesIndex != "8.7" {
		t.Fatalf("expected douban rating 8.7, got %q", got.SeriesIndex)
	}
}
