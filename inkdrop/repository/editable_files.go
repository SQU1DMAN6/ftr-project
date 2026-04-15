package repository

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

type EditableFileKind string

const (
	EditableFileKindUnknown  EditableFileKind = "unknown"
	EditableFileKindText     EditableFileKind = "text"
	EditableFileKindDocument EditableFileKind = "document"
)

var errUnsupportedEditableFile = errors.New("unsupported editable file type")

func CreateEmptyEditableFile(userName, repoName, workingDir, fileName, fileType, extension string) (string, error) {
	finalName, err := buildEditableFileName(fileName, extension)
	if err != nil {
		return "", err
	}

	data, err := buildEmptyEditableFileContent(fileType)
	if err != nil {
		return "", err
	}

	repoPath := normalizeWorkingDir(path.Join(workingDir, finalName))
	if _, err := WriteFileAtRepoPath(userName, repoName, repoPath, data, false); err != nil {
		return "", err
	}

	return finalName, nil
}

func DetectEditableFileKind(filePath string) (EditableFileKind, error) {
	if isDOCXDocumentFile(filePath) {
		return EditableFileKindDocument, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return EditableFileKindUnknown, err
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return EditableFileKindUnknown, nil
	}

	return EditableFileKindText, nil
}

func buildEditableFileName(fileName, extension string) (string, error) {
	baseName := strings.TrimSpace(fileName)
	if baseName == "" || baseName == "." || baseName == ".." {
		return "", errors.New("file name is required")
	}
	if strings.ContainsAny(baseName, "/\\") {
		return "", errors.New("file name cannot include path separators")
	}

	ext := strings.TrimSpace(extension)
	if ext != "" {
		if strings.ContainsAny(ext, "/\\ \t\r\n") {
			return "", errors.New("extension contains invalid characters")
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		if ext == "." {
			ext = ""
		}
	}

	if ext != "" && strings.HasSuffix(strings.ToLower(baseName), strings.ToLower(ext)) {
		return baseName, nil
	}
	return baseName + ext, nil
}

func buildEmptyEditableFileContent(fileType string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(fileType)) {
	case "", "text", "plain_text", "plain-text":
		return []byte{}, nil
	case "document", "docx":
		return buildEmptyDOCXDocument()
	default:
		return nil, errUnsupportedEditableFile
	}
}

func isDOCXDocumentFile(filePath string) bool {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return false
	}
	defer reader.Close()

	requiredEntries := map[string]bool{
		"[Content_Types].xml": false,
		"_rels/.rels":         false,
		"word/document.xml":   false,
	}
	for _, file := range reader.File {
		if _, ok := requiredEntries[file.Name]; ok {
			requiredEntries[file.Name] = true
		}
	}
	for _, present := range requiredEntries {
		if !present {
			return false
		}
	}
	return true
}

func buildEmptyDOCXDocument() ([]byte, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	files := []struct {
		name string
		body string
	}{
		{
			name: "[Content_Types].xml",
			body: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
  <Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
  <Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
  <Override PartName="/word/webSettings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.webSettings+xml"/>
  <Override PartName="/word/fontTable.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml"/>
  <Override PartName="/word/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
</Types>`,
		},
		{
			name: "_rels/.rels",
			body: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`,
		},
		{
			name: "docProps/app.xml",
			body: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"
  xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
  <Application>InkDrop</Application>
</Properties>`,
		},
		{
			name: "docProps/core.xml",
			body: fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
  xmlns:dc="http://purl.org/dc/elements/1.1/"
  xmlns:dcterms="http://purl.org/dc/terms/"
  xmlns:dcmitype="http://purl.org/dc/dcmitype/"
  xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <dc:title></dc:title>
  <dc:creator>InkDrop</dc:creator>
  <cp:lastModifiedBy>InkDrop</cp:lastModifiedBy>
  <dcterms:created xsi:type="dcterms:W3CDTF">%s</dcterms:created>
  <dcterms:modified xsi:type="dcterms:W3CDTF">%s</dcterms:modified>
</cp:coreProperties>`, now, now),
		},
		{
			name: "word/document.xml",
			body: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:wpc="http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas"
  xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"
  xmlns:o="urn:schemas-microsoft-com:office:office"
  xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
  xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"
  xmlns:v="urn:schemas-microsoft-com:vml"
  xmlns:wp14="http://schemas.microsoft.com/office/word/2010/wordprocessingDrawing"
  xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
  xmlns:w10="urn:schemas-microsoft-com:office:word"
  xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
  xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"
  xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml"
  xmlns:wpg="http://schemas.microsoft.com/office/word/2010/wordprocessingGroup"
  xmlns:wpi="http://schemas.microsoft.com/office/word/2010/wordprocessingInk"
  xmlns:wne="http://schemas.microsoft.com/office/word/2006/wordml"
  xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
  mc:Ignorable="w14 w15 wp14">
  <w:body>
    <w:p/>
    <w:sectPr>
      <w:pgSz w:w="12240" w:h="15840"/>
      <w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="708" w:footer="708" w:gutter="0"/>
      <w:cols w:space="708"/>
      <w:docGrid w:linePitch="360"/>
    </w:sectPr>
  </w:body>
</w:document>`,
		},
		{
			name: "word/_rels/document.xml.rels",
			body: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
		},
		{
			name: "word/styles.xml",
			body: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:default="1" w:styleId="Normal">
    <w:name w:val="Normal"/>
    <w:qFormat/>
  </w:style>
</w:styles>`,
		},
		{
			name: "word/settings.xml",
			body: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:zoom w:percent="100"/>
</w:settings>`,
		},
		{
			name: "word/webSettings.xml",
			body: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:webSettings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"/>`,
		},
		{
			name: "word/fontTable.xml",
			body: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:fonts xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:font w:name="Calibri"/>
</w:fonts>`,
		},
		{
			name: "word/theme/theme1.xml",
			body: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme">
  <a:themeElements>
    <a:clrScheme name="Office">
      <a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1>
      <a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>
      <a:dk2><a:srgbClr val="1F497D"/></a:dk2>
      <a:lt2><a:srgbClr val="EEECE1"/></a:lt2>
      <a:accent1><a:srgbClr val="4F81BD"/></a:accent1>
      <a:accent2><a:srgbClr val="C0504D"/></a:accent2>
      <a:accent3><a:srgbClr val="9BBB59"/></a:accent3>
      <a:accent4><a:srgbClr val="8064A2"/></a:accent4>
      <a:accent5><a:srgbClr val="4BACC6"/></a:accent5>
      <a:accent6><a:srgbClr val="F79646"/></a:accent6>
      <a:hlink><a:srgbClr val="0000FF"/></a:hlink>
      <a:folHlink><a:srgbClr val="800080"/></a:folHlink>
    </a:clrScheme>
    <a:fontScheme name="Office">
      <a:majorFont>
        <a:latin typeface="Calibri Light"/>
      </a:majorFont>
      <a:minorFont>
        <a:latin typeface="Calibri"/>
      </a:minorFont>
    </a:fontScheme>
    <a:fmtScheme name="Office">
      <a:fillStyleLst/>
      <a:lnStyleLst/>
      <a:effectStyleLst/>
      <a:bgFillStyleLst/>
    </a:fmtScheme>
  </a:themeElements>
</a:theme>`,
		},
	}

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, file := range files {
		entryWriter, err := writer.Create(file.name)
		if err != nil {
			writer.Close()
			return nil, err
		}
		if _, err := entryWriter.Write([]byte(file.body)); err != nil {
			writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
