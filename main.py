#!/opt/homebrew/bin/python3

import argparse
import csv
import os.path
import sys

import fitz


parser = argparse.ArgumentParser()
parser.add_argument("pdfs", metavar="PDFs", type=str, nargs="+", help="the PDFs to extact highlights from")
# Probably need to make the highlight regions slightly bigger to *fully* contain the text regions
parser.add_argument("--scale-w", type=float, default=1.0, help="scale the highlight rect's width")
parser.add_argument("--scale-h", type=float, default=1.0, help="scale the highlight rect's height")
# For visualizing the highlight rects that PyMuPDF sees compared to what you see in the PDF
parser.add_argument(
    "--vis",
    action="store_true",
    help="draw the highlight rects for debugging; creates viz_*.pdf files side-by-side with the oringials",
)
args = parser.parse_args()

Writer = csv.writer(sys.stdout)
Writer.writerow(["Filename", "Page_num", "Highlighted_text"])

for input_path in args.pdfs:
    doc: fitz.Document = fitz.open(input_path)

    for page_idx in range(len(doc)):
        page_num = page_idx + 1

        page: fitz.Page = doc[page_idx]
        annot: fitz.Annot = page.first_annot
        while annot:
            if annot.type[0] != fitz.PDF_ANNOT_HIGHLIGHT:
                # skipping non-highlight
                continue

            texts = []
            # Iterate groups of 4 vertices, to get multiple regions for a single "highlight"
            for i in range(0, len(annot.vertices), 4):
                r = fitz.Quad(annot.vertices[i : i + 4]).rect

                # Get center of rectangle, then,
                # move rect to page origin (0, 0), scale up, then move back
                rcX, rcY = (r.x0 + r.width / 2), (r.y0 + r.height / 2)
                m = fitz.Matrix(1, 0, 0, 1, -rcX, -rcY)
                m.concat(m, fitz.Matrix(args.scale_w, args.scale_h))
                m.concat(m, fitz.Matrix(1, 0, 0, 1, rcX, rcY))

                r = r.transform(m)

                if args.vis:
                    # Draw a red box to visualize the hightlight rect's area (text)
                    page.draw_rect(r, width=1.5, color=(1, 0, 0))

                # Finally get text inside rect
                texts.append(page.get_textbox(r))

            all_text = " ".join([x.strip() for x in texts])

            Writer.writerow([input_path, page_num, all_text])

            annot = annot.next

    if args.vis:
        head, tail = os.path.split(input_path)
        viz_name = os.path.join(head, "viz_" + tail)
        doc.save(viz_name)
