package doc

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/spf13/afero"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func translate(in string) string {
	return strings.Replace(in, "-", "\\-", -1)
}

func TestAferoFs(t *testing.T) {
	oldDocFs := GetFS()
	defer func() {
		SetFS(oldDocFs)
	}()
	SetFS(&afero.Afero{Fs: afero.NewMemMapFs()})

	err := docFs.MkdirAll("/__cobra-tests/manpages", os.ModeDir)
	if err != nil {
		t.Errorf("Expected no error, but got: %s", err)
	}

	f, err := docFs.Create("/__cobra-tests/manpages/page.3")
	if err != nil {
		t.Errorf("Expected to create file /__cobra-tests/manpages/page.3 without error, but got: %s", err)
	}
	defer f.Close()

	writeCount, err := f.WriteString("manpage content")
	if err != nil {
		t.Errorf("Expected to write to file /__cobra-tests/manpages/page.3 without error, but got: %s", err)
	}

	if writeCount != len("manpage content") {
		t.Errorf("Expected to write %d bytes in file, but %d really written", len("manpage content"), writeCount)
	}

	if _, err := docFs.Stat("/__cobra-tests/manpages"); os.IsNotExist(err) {
		t.Errorf("Exprected /__cobra-tests/manpages to exists")
	}

	SetFS(oldDocFs)
	if _, err := docFs.Stat("/__cobra-tests/manpages"); !os.IsNotExist(err) {
		t.Errorf("Exprected /__cobra-tests/manpages to not exists anymore")
	}
}

func TestGenManDoc(t *testing.T) {
	header := &GenManHeader{
		Title:   "Project",
		Section: "2",
	}

	// We generate on a subcommand so we have both subcommands and parents
	buf := new(bytes.Buffer)
	if err := GenMan(echoCmd, header, buf); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	// Make sure parent has - in CommandPath() in SEE ALSO:
	parentPath := echoCmd.Parent().CommandPath()
	dashParentPath := strings.Replace(parentPath, " ", "-", -1)
	expected := translate(dashParentPath)
	expected = expected + "(" + header.Section + ")"
	checkStringContains(t, output, expected)

	checkStringContains(t, output, translate(echoCmd.Name()))
	checkStringContains(t, output, translate(echoCmd.Name()))
	checkStringContains(t, output, "boolone")
	checkStringContains(t, output, "rootflag")
	checkStringContains(t, output, translate(rootCmd.Name()))
	checkStringContains(t, output, translate(echoSubCmd.Name()))
	checkStringOmits(t, output, translate(deprecatedCmd.Name()))
	checkStringContains(t, output, translate("Auto generated"))
}

func TestGenManNoHiddenParents(t *testing.T) {
	header := &GenManHeader{
		Title:   "Project",
		Section: "2",
	}

	// We generate on a subcommand so we have both subcommands and parents
	for _, name := range []string{"rootflag", "strtwo"} {
		f := rootCmd.PersistentFlags().Lookup(name)
		f.Hidden = true
		defer func() { f.Hidden = false }()
	}
	buf := new(bytes.Buffer)
	if err := GenMan(echoCmd, header, buf); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	// Make sure parent has - in CommandPath() in SEE ALSO:
	parentPath := echoCmd.Parent().CommandPath()
	dashParentPath := strings.Replace(parentPath, " ", "-", -1)
	expected := translate(dashParentPath)
	expected = expected + "(" + header.Section + ")"
	checkStringContains(t, output, expected)

	checkStringContains(t, output, translate(echoCmd.Name()))
	checkStringContains(t, output, translate(echoCmd.Name()))
	checkStringContains(t, output, "boolone")
	checkStringOmits(t, output, "rootflag")
	checkStringContains(t, output, translate(rootCmd.Name()))
	checkStringContains(t, output, translate(echoSubCmd.Name()))
	checkStringOmits(t, output, translate(deprecatedCmd.Name()))
	checkStringContains(t, output, translate("Auto generated"))
	checkStringOmits(t, output, "OPTIONS INHERITED FROM PARENT COMMANDS")
}

func TestGenManNoGenTag(t *testing.T) {
	echoCmd.DisableAutoGenTag = true
	defer func() { echoCmd.DisableAutoGenTag = false }()

	header := &GenManHeader{
		Title:   "Project",
		Section: "2",
	}

	// We generate on a subcommand so we have both subcommands and parents
	buf := new(bytes.Buffer)
	if err := GenMan(echoCmd, header, buf); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	unexpected := translate("#HISTORY")
	checkStringOmits(t, output, unexpected)
}

func TestGenManSeeAlso(t *testing.T) {
	rootCmd := &cobra.Command{Use: "root", Run: emptyRun}
	aCmd := &cobra.Command{Use: "aaa", Run: emptyRun, Hidden: true} // #229
	bCmd := &cobra.Command{Use: "bbb", Run: emptyRun}
	cCmd := &cobra.Command{Use: "ccc", Run: emptyRun}
	rootCmd.AddCommand(aCmd, bCmd, cCmd)

	buf := new(bytes.Buffer)
	header := &GenManHeader{}
	if err := GenMan(rootCmd, header, buf); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(buf)

	if err := assertLineFound(scanner, ".SH SEE ALSO"); err != nil {
		t.Fatalf("Couldn't find SEE ALSO section header: %v", err)
	}
	if err := assertNextLineEquals(scanner, ".PP"); err != nil {
		t.Fatalf("First line after SEE ALSO wasn't break-indent: %v", err)
	}
	if err := assertNextLineEquals(scanner, `\fBroot\-bbb(1)\fP, \fBroot\-ccc(1)\fP`); err != nil {
		t.Fatalf("Second line after SEE ALSO wasn't correct: %v", err)
	}
}

func TestManPrintFlagsHidesShortDeperecated(t *testing.T) {
	c := &cobra.Command{}
	c.Flags().StringP("foo", "f", "default", "Foo flag")
	c.Flags().MarkShorthandDeprecated("foo", "don't use it no more")

	buf := new(bytes.Buffer)
	manPrintFlags(buf, c.Flags())

	got := buf.String()
	expected := "**--foo**=\"default\"\n\tFoo flag\n\n"
	if got != expected {
		t.Errorf("Expected %v, got %v", expected, got)
	}
}

func TestGenManCommands(t *testing.T) {
	header := &GenManHeader{
		Title:   "Project",
		Section: "2",
	}

	// Root command
	buf := new(bytes.Buffer)
	if err := GenMan(rootCmd, header, buf); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	checkStringContains(t, output, ".SH COMMANDS")
	checkStringMatch(t, output, "\\\\fBecho\\\\fP\n[ \t]+Echo anything to the screen\n[ \t]+See \\\\fBroot\\\\-echo\\(2\\)\\\\fP\\\\&\\.")
	checkStringOmits(t, output, ".PP\n\\fBprint\\fP\n")

	// Echo command
	buf = new(bytes.Buffer)
	if err := GenMan(echoCmd, header, buf); err != nil {
		t.Fatal(err)
	}
	output = buf.String()

	checkStringContains(t, output, ".SH COMMANDS")
	checkStringMatch(t, output, "\\\\fBtimes\\\\fP\n[ \t]+Echo anything to the screen more times\n[ \t]+See \\\\fBroot\\\\-echo\\\\-times\\(2\\)\\\\fP\\\\&\\.")
	checkStringMatch(t, output, "\\\\fBechosub\\\\fP\n[ \t]+second sub command for echo\n[ \t]+See \\\\fBroot\\\\-echo\\\\-echosub\\(2\\)\\\\fP\\\\&\\.")
	checkStringOmits(t, output, ".PP\n\\fBdeprecated\\fP\n")

	// Time command as echo's subcommand
	buf = new(bytes.Buffer)
	if err := GenMan(timesCmd, header, buf); err != nil {
		t.Fatal(err)
	}
	output = buf.String()

	checkStringOmits(t, output, ".SH COMMANDS")
}

func TestGenManTree(t *testing.T) {
	c := &cobra.Command{Use: "do [OPTIONS] arg1 arg2"}
	header := &GenManHeader{Section: "2"}
	tmpdir, err := ioutil.TempDir("", "test-gen-man-tree")
	if err != nil {
		t.Fatalf("Failed to create tmpdir: %s", err.Error())
	}
	defer os.RemoveAll(tmpdir)

	if err := GenManTree(c, header, tmpdir); err != nil {
		t.Fatalf("GenManTree failed: %s", err.Error())
	}

	if _, err := os.Stat(filepath.Join(tmpdir, "do.2")); err != nil {
		t.Fatalf("Expected file 'do.2' to exist")
	}

	if header.Title != "" {
		t.Fatalf("Expected header.Title to be unmodified")
	}
}

func assertLineFound(scanner *bufio.Scanner, expectedLine string) error {
	for scanner.Scan() {
		line := scanner.Text()
		if line == expectedLine {
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan failed: %s", err)
	}

	return fmt.Errorf("hit EOF before finding %v", expectedLine)
}

func assertNextLineEquals(scanner *bufio.Scanner, expectedLine string) error {
	if scanner.Scan() {
		line := scanner.Text()
		if line == expectedLine {
			return nil
		}
		return fmt.Errorf("got %v, not %v", line, expectedLine)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan failed: %v", err)
	}

	return fmt.Errorf("hit EOF before finding %v", expectedLine)
}

func BenchmarkGenManToFile(b *testing.B) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := GenMan(rootCmd, nil, file); err != nil {
			b.Fatal(err)
		}
	}
}
