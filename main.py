#!/opt/homebrew/bin/python3

import csv
import os.path
import sys

import fitz

# Need to make the highlight regions slightly bigger to *fully*
# contain the text regions
SCALE_X = 1.001
SCALE_Y = 1.001

# For visualizing the highlight rects that PyMuPDF sees compared to what you see in the PDF
VISUALIZE = True

Writer = csv.writer(sys.stdout)
Writer.writerow(["Filename", "Page_num", "Highlighted_text"])

for input_path in sys.argv[1:]:
    doc: fitz.Document = fitz.open(input_path)

    for page_idx in range(len(doc)):
        page_num = page_idx + 1

        page: fitz.Page = doc[page_idx]
        annot: fitz.Annot = page.firstAnnot
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
                m.concat(m, fitz.Matrix(SCALE_X, SCALE_Y))
                m.concat(m, fitz.Matrix(1, 0, 0, 1, rcX, rcY))

                r = r.transform(m)

                if VISUALIZE:
                    # Draw a red box to visualize the hightlight rect's area (text)
                    page.draw_rect(r, width=1.5, color=(1, 0, 0))

                # Finally get text inside rect
                texts.append(page.get_textbox(r))

            all_text = " ".join([x.strip() for x in texts])

            Writer.writerow([input_path, page_num, all_text])

            annot = annot.next

    if VISUALIZE:
        head, tail = os.path.split(input_path)
        viz_name = os.path.join(head, "viz_" + tail)
        doc.save(viz_name)
