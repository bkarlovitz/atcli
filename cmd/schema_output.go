package cmd

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"atcli/internal/attio"
)

func printObjects(out io.Writer, objects []attio.Object) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "API SLUG\tOBJECT ID\tSINGULAR\tPLURAL")
	for _, object := range objects {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", object.APISlug, object.ID.ObjectID, object.SingularNoun, object.PluralNoun)
	}
	_ = w.Flush()
}

func printLists(out io.Writer, lists []attio.List) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "API SLUG\tLIST ID\tNAME\tPARENT OBJECT")
	for _, list := range lists {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", list.APISlug, list.ID.ListID, list.Name, strings.Join(list.ParentObject, ", "))
	}
	_ = w.Flush()
}

func printAttributes(out io.Writer, attributes []attio.Attribute) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "API SLUG\tATTRIBUTE ID\tTITLE\tTYPE\tWRITABLE\tEDITABLE\tREQUIRED\tUNIQUE\tMULTISELECT\tARCHIVED")
	for _, attribute := range attributes {
		_, _ = fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			attribute.APISlug,
			attribute.ID.AttributeID,
			attribute.Title,
			attribute.Type,
			yesNo(attribute.IsWritable),
			optionalYesNo(attribute.IsEditable),
			yesNo(attribute.IsRequired),
			yesNo(attribute.IsUnique),
			yesNo(attribute.IsMultiselect),
			yesNo(attribute.IsArchived),
		)
	}
	_ = w.Flush()
}

func visibleAttributes(attributes []attio.Attribute, includeArchived bool) []attio.Attribute {
	if includeArchived {
		return attributes
	}

	visible := make([]attio.Attribute, 0, len(attributes))
	for _, attribute := range attributes {
		if !attribute.IsArchived {
			visible = append(visible, attribute)
		}
	}
	return visible
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func optionalYesNo(value *bool) string {
	if value == nil {
		return ""
	}
	return yesNo(*value)
}
