/*
 * Extract highlighted text.
 *
 * Run as: go run list_highlights.go input.pdf [input2.pdf, ...]
 */

package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unidoc/unipdf/v3/annotator"
	"github.com/unidoc/unipdf/v3/common/license"
	"github.com/unidoc/unipdf/v3/core"
	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
)

// Cmd-line init
func init() {
	// Make sure to load your metered License API key prior to using the library.
	// If you need a key, you can sign up and create a free one at https://cloud.unidoc.io
	err := license.SetMeteredKey(os.Getenv(`UNIDOC_LICENSE_API_KEY`))
	if err != nil {
		panic(err)
	}
}

// UniDOC Playground init
//  func init() {
// 	 os.Args = []string{"playground", "one_page.pdf", "two_pages.pdf"}
//  }

// Need to make the highlight regions slightly smaller in height, so as not
// overlap with text below the highlight
const SCALE_W = 1.0
const SCALE_H = 0.76 // 0.76 is the maximum to get the right text for Alice

// For visualizing the highlight rects that UniPDF sees compared to what you see in the PDF
const VISUALIZE = true

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run list_highlights.go input.pdf [input2.pdf, ...]")
		os.Exit(1)
	}

	// Create CSV writer
	csvW := csv.NewWriter(os.Stdout)
	csvW.Write([]string{"Filename", "Page_num", "Highlighted_text"})

	// Iterate input PDFs
	for _, inputPath := range os.Args[1:len(os.Args)] {
		pdfReader, f, err := model.NewPdfReaderFromFile(inputPath, nil)
		if err != nil {
			fmt.Printf("error: could not create PdfReader for %q: %q\n", inputPath, err)
			continue
		}
		defer f.Close()

		numPages, err := pdfReader.GetNumPages()
		if err != nil {
			fmt.Printf("error: could not get number of pages of %q: %q\n", inputPath, err)
			continue
		}

		// Map a page number to slice of Annotator rects (for visualizing/debugging)
		pageVizRectsMap := make(map[int][]annotator.RectangleAnnotationDef)

		// Iterate pages
		for i := 1; i <= numPages; i++ {
			page, err := pdfReader.GetPage(i)
			if err != nil {
				fmt.Printf("error: could not get page %d of %q: %q\n", i, inputPath, err)
				continue
			}

			ex, err := extractor.New(page)
			if err != nil {
				fmt.Printf("error: could not create extractor for page %d of %q: %q\n", i, inputPath, err)
				continue
			}

			pageText, _, _, err := ex.ExtractPageText()
			if err != nil {
				fmt.Printf("error: could not extract text on page %d of %q: %q\n", i, inputPath, err)
				continue
			}

			annotations, err := page.GetAnnotations()
			if err != nil {
				fmt.Printf("error: could not get annotations on page %d of %q: %q\n", i, inputPath, err)
				continue
			}

			vizRects := make([]annotator.RectangleAnnotationDef, 0)

			for _, annotation := range annotations {
				highlight, isHL := annotation.GetContext().(*model.PdfAnnotationHighlight)
				if !isHL {
					// skip non-highlight annotation
					continue
				}

				objArr, isObjArr := highlight.QuadPoints.(*core.PdfObjectArray)
				if !isObjArr {
					// skip non-ObjectArray
					continue
				}

				quadPts, err := objArr.ToFloat64Array()
				if err != nil {
					fmt.Printf("error: could not convert PdfObjectArrary %q to Float64Array: %q\n", objArr, err)
					continue
				}
				if len(quadPts)%8 != 0 {
					fmt.Printf("error: needs QuadPoints to be a multiple of 8, its length is %d\n", len(quadPts))
					continue
				}

				// Iterate sets of QuadPoints to get individual highlight rects, and extract text for the rect
				var allText []string
				var llx, lly, urx, ury float64
				var w, h, cx, cy float64
				for i := 0; i < len(quadPts); i += 8 {
					pts := quadPts[i : i+8]

					// Original diagonal corners of rect, and width and height
					llx, lly, urx, ury = pts[4], pts[5], pts[2], pts[3]
					w, h = (urx - llx), (ury - lly)
					// Get center
					cx, cy = llx+(w/2), lly+(h/2)
					// Scale sides
					w, h = w*SCALE_W, h*SCALE_H
					// Recompute diagonal corners
					llx, lly, urx, ury = cx-(w/2), cy-(h/2), cx+(w/2), cy+(h/2)

					rect := model.PdfRectangle{Llx: llx, Lly: lly, Urx: urx, Ury: ury}
					pageText.ApplyArea(rect)
					text := pageText.Text()
					text = strings.TrimSpace(text)
					if len(text) > 0 {
						allText = append(allText, text)
					}

					if VISUALIZE {
						rectDef := annotator.RectangleAnnotationDef{}
						rectDef.X = llx
						rectDef.Y = lly
						rectDef.Width = w
						rectDef.Height = h
						vizRects = append(vizRects, rectDef)
					}
				}

				if len(allText) > 0 {
					text := strings.Join(allText, " ")
					csvW.Write([]string{inputPath, fmt.Sprintf("%d", i), text})
				}
			}

			if len(vizRects) > 0 {
				pageVizRectsMap[i] = vizRects
			}
		}

		if VISUALIZE {
			opt := &model.ReaderToWriterOpts{
				// Callback is executed for every page, with the page as pageNum, during the Reader-to-Writer conversion
				PageProcessCallback: func(pageNum int, page *model.PdfPage) error {
					// See if this page has any viz/debug annotation rects
					rectDefs, ok := pageVizRectsMap[pageNum]
					if !ok {
						return nil
					}
					// Add rects to page
					for _, rectDef := range rectDefs {
						rectDef.Opacity = 1
						rectDef.BorderEnabled = true
						rectDef.BorderWidth = 1.5
						rectDef.BorderColor = model.NewPdfColorDeviceRGB(1, 0, 0) // Red border
						rectAnnotation, err := annotator.CreateRectangleAnnotation(rectDef)
						if err != nil {
							return err
						}
						page.AddAnnotation(rectAnnotation)
					}
					return nil
				},
			}

			pdfWriter, err := pdfReader.ToWriter(opt)
			if err != nil {
				fmt.Printf("error: could not create Writer: %q\n", err)
				os.Exit(1)
			}

			dir, file := filepath.Split(inputPath)
			outputPath := filepath.Join(dir, "viz_"+file)
			err = pdfWriter.WriteToFile(outputPath)
			if err != nil {
				fmt.Printf("error: could not write VIZ PDF to %q: %q\n", outputPath, err)
				return
			}
		}
	}
	csvW.Flush()
}
