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

func TestMergeScrapedMetadata_UsesScrapedTitleWhenOriginalMissing(t *testing.T) {
	original := Metadata{}
	scraped := Metadata{Title: "三体"}

	got := mergeScrapedMetadata(original, scraped)
	if got.Title != "三体" {
		t.Fatalf("expected scraped title when original is empty, got %q", got.Title)
	}
}

func TestParseDoubanBookPageExtractsAuthorAndSeries(t *testing.T) {
	body := `
<html>
  <head>
    <meta property="og:title" content="三体"/>
    <meta property="og:description" content="科幻小说"/>
  </head>
  <body>
    <div id="info">
      <span class="pl">作者:</span> <a href="/author/1">刘慈欣</a><br/>
      <span class="pl">丛书:</span> <a href="/series/1">地球往事三部曲</a><br/>
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
