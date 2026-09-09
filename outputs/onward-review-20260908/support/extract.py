import datetime
import json
import re
from pathlib import Path

import openpyxl
from docx import Document
from docx.table import Table
from docx.text.paragraph import Paragraph

SOURCE = Path('D:/桌面/Onward_业务功能对照与实施排期_最终版_20260907.xlsx')
DOC = Path('D:/桌面/Onward-HelpDesk-Platform-内部开发需求与总体设计-v0.1.0.docx')
workbook = openpyxl.load_workbook(SOURCE, data_only=False)
document = Document(DOC)
requirements = []
sections = {}
section = ''
for block in document.element.body:
    if block.tag.endswith('}p'):
        p = Paragraph(block, document)
        if p.style.name.startswith('Heading'):
            section = p.text
        elif p.text:
            sections.setdefault(section, []).append(p.text)
    elif block.tag.endswith('}tbl'):
        table = Table(block, document)
        for row in table.rows:
            cells = [c.text for c in row.cells]
            sections.setdefault(section, []).append(' | '.join(cells))
            if re.fullmatch(r'[A-Z]+-\d{2,3}', cells[0]):
                requirements.append({'id': cells[0], 'section': section, 'level': cells[1] if len(cells) > 2 else '约束', 'text': cells[-1] if len(cells) > 2 else cells[1], 'cells': cells})

def scalar(value):
    if isinstance(value, (datetime.datetime, datetime.date)):
        return {'date': value.isoformat()}
    return value

sheets = {}
for sheet in workbook:
    sheets[sheet.title] = {
        'rows': [[scalar(c.value) for c in row] for row in sheet],
        'merges': [str(r) for r in sheet.merged_cells.ranges],
        'filter': sheet.auto_filter.ref,
        'freeze': sheet.freeze_panes,
    }
print(json.dumps({'sheets': sheets, 'requirements': requirements, 'sections': sections}, ensure_ascii=False))
