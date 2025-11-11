package pdf

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/jung-kurt/gofpdf"

	"links_service/internal/entity"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) Generate(records []*entity.LinkRecord) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetCreator("links_service", true)
	pdf.SetAuthor("links_service", true)
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 18)
	pdf.Cell(40, 12, "Link status report")
	pdf.Ln(14)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 8, fmt.Sprintf("Generated at: %s", time.Now().UTC().Format(time.RFC3339)))
	pdf.Ln(10)

	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })

	for idx, record := range records {
		if idx > 0 {
			pdf.Ln(4)
		}

		pdf.SetFont("Arial", "B", 14)
		pdf.Cell(0, 8, fmt.Sprintf("Request #%d", record.ID))
		pdf.Ln(8)

		pdf.SetFont("Arial", "", 11)
		pdf.Cell(0, 6, fmt.Sprintf("Requested at: %s", record.RequestedAt.Format(time.RFC3339)))
		pdf.Ln(6)
		if record.CompletedAt != nil {
			pdf.Cell(0, 6, fmt.Sprintf("Completed at: %s", record.CompletedAt.Format(time.RFC3339)))
			pdf.Ln(6)
		}

		pdf.SetFont("Arial", "B", 12)
		pdf.CellFormat(110, 8, "Link", "1", 0, "", false, 0, "")
		pdf.CellFormat(60, 8, "Status", "1", 1, "", false, 0, "")

		pdf.SetFont("Arial", "", 12)
		for _, link := range record.Links {
			status := record.Statuses[link]
			pdf.CellFormat(110, 8, link, "1", 0, "", false, 0, "")
			pdf.CellFormat(60, 8, string(status), "1", 1, "", false, 0, "")
		}
	}

	buf := bytes.Buffer{}
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
