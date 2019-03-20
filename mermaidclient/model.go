// Copyright (C) 2017 ScyllaDB

package mermaidclient

import (
	"fmt"
	"io"
	"text/template"

	"github.com/scylladb/mermaid/mermaidclient/internal/models"
	"github.com/scylladb/mermaid/mermaidclient/table"
)

// TableRenderer is the interface that components need to implement
// if they can render themselves as tables.
type TableRenderer interface {
	Render(io.Writer) error
}

// Cluster is cluster.Cluster representation.
type Cluster = models.Cluster

// ClusterSlice is []*cluster.Cluster representation.
type ClusterSlice []*models.Cluster

// Render renders ClusterSlice in a tabular format.
func (cs ClusterSlice) Render(w io.Writer) error {
	t := table.New("cluster id", "name", "ssh user")
	for _, c := range cs {
		t.AddRow(c.ID, c.Name, c.SSHUser)
	}
	if _, err := w.Write([]byte(t.String())); err != nil {
		return err
	}

	return nil
}

// Task is a sched.Task representation.
type Task = models.Task

func makeTaskUpdate(t *Task) *models.TaskUpdate {
	return &models.TaskUpdate{
		Type:       t.Type,
		Enabled:    t.Enabled,
		Name:       t.Name,
		Schedule:   t.Schedule,
		Tags:       t.Tags,
		Properties: t.Properties,
	}
}

// Target is a representing results of dry running repair task.
type Target struct {
	models.RepairTarget
}

const targetTemplate = `{{ if ne .TokenRanges "dcpr" -}}
Token Ranges: {{ .TokenRanges }}
{{ end -}}
{{ if .Host -}}
Host: {{ .Host }}
{{ end -}}
{{ if .WithHosts -}}
With Hosts:
{{ range .WithHosts -}}
  - {{ . }}
{{ end -}}
{{ end -}}
Data Centers:
{{ range .Dc }}  - {{ . }}
{{ end -}}
{{ range .Units }}Keyspace: {{ .Keyspace }}
{{- if .AllTables }}
  (all tables)
{{ else -}}
{{ range .Tables }}
  - {{ . }}
{{ end -}}
{{- end -}}
{{ end }}`

// Render implements Renderer interface.
func (t Target) Render(w io.Writer) error {
	temp := template.Must(template.New("target").Parse(targetTemplate))
	return temp.Execute(w, t)
}

// ExtendedTask is a representation of sched.Task with additional fields from sched.Run.
type ExtendedTask = models.ExtendedTask

// ExtendedTaskSlice is a representation of a slice of sched.Task with additional fields from sched.Run.
type ExtendedTaskSlice = []*models.ExtendedTask

// ExtendedTasks is a representation of []*sched.Task with additional fields from sched.Run.
type ExtendedTasks struct {
	ExtendedTaskSlice
	All bool
}

// Render renders ExtendedTasks in a tabular format.
func (et ExtendedTasks) Render(w io.Writer) error {
	p := table.New("task", "next run", "ret.", "properties", "status")
	for _, t := range et.ExtendedTaskSlice {
		id := fmt.Sprint(t.Type, "/", t.ID)
		if et.All && !t.Enabled {
			id = "*" + id
		}
		r := FormatTime(t.NextActivation)
		if t.Schedule.Interval != "" {
			r += fmt.Sprint(" (+", t.Schedule.Interval, ")")
		}
		s := t.Status
		p.AddRow(id, r, formatRetries(t.Schedule.NumRetries, t.Failures), dumpMap(t.Properties.(map[string]interface{})), s)
	}
	fmt.Fprint(w, p)
	return nil
}

// Schedule is a sched.Schedule representation.
type Schedule = models.Schedule

// TaskRun is a sched.TaskRun representation.
type TaskRun = models.TaskRun

// TaskRunSlice is a []*sched.TaskRun representation.
type TaskRunSlice []*TaskRun

// Render renders TaskRunSlice in a tabular format.
func (tr TaskRunSlice) Render(w io.Writer) error {
	t := table.New("id", "start time", "end time", "duration", "status")
	for _, r := range tr {
		s := r.Status
		t.AddRow(r.ID, FormatTime(r.StartTime), FormatTime(r.EndTime), FormatDuration(r.StartTime, r.EndTime), s)
	}
	if _, err := w.Write([]byte(t.String())); err != nil {
		return err
	}
	return nil
}

// RepairProgress contains shard progress info.
type RepairProgress struct {
	*models.TaskRunRepairProgress
	Detailed bool
}

// Render renders *RepairProgress in a tabular format.
func (rp RepairProgress) Render(w io.Writer) error {
	if err := rp.addHeader(w); err != nil {
		return err
	}

	if rp.Progress != nil {
		t := table.New()
		rp.addRepairUnitProgress(t)
		if _, err := io.WriteString(w, t.String()); err != nil {
			return err
		}
	}

	if rp.Progress != nil && rp.Detailed {
		d := table.New()
		for i, u := range rp.Progress.Units {
			if i > 0 {
				d.AddSeparator()
			}
			d.AddRow(u.Unit.Keyspace, "shard", "progress", "segment_count", "segment_success", "segment_error")
			if len(u.Nodes) > 0 {
				d.AddSeparator()
				rp.addRepairUnitDetailedProgress(d, u)
			}
		}
		if _, err := w.Write([]byte(d.String())); err != nil {
			return err
		}
	}
	return nil
}

var progressTemplate = `{{ with .Run }}Status:		{{ .Status }}
{{- if .Cause }}
Cause:		{{ .Cause }}
{{- end }}
{{- if not (isZero .StartTime) }}
Start time:	{{ FormatTime .StartTime }}
{{- end -}}
{{- if not (isZero .EndTime) }}
End time:	{{ FormatTime .EndTime }}
{{- end }}
Duration:	{{ FormatDuration .StartTime .EndTime }}
{{ end -}}
{{ with .Progress }}Progress:	{{ FormatProgress .PercentComplete .PercentFailed }}
{{- if .Ranges }}
Token ranges:	{{ .Ranges }}
{{ end -}}
{{ if .Dcs }}
Datacenters:	{{ range .Dcs }}
  - {{ . }}
{{- end }}
{{ end -}}
{{ else }}Progress:	0%
{{ end }}`

func (rp RepairProgress) addHeader(w io.Writer) error {
	temp := template.Must(template.New("repair_progress").Funcs(template.FuncMap{
		"isZero":         isZero,
		"FormatTime":     FormatTime,
		"FormatDuration": FormatDuration,
		"FormatProgress": FormatProgress,
	}).Parse(progressTemplate))
	return temp.Execute(w, rp)
}

func (rp RepairProgress) addRepairUnitProgress(t *table.Table) {
	for _, u := range rp.Progress.Units {
		t.AddRow(u.Unit.Keyspace, FormatProgress(u.PercentComplete, u.PercentFailed))
	}
}

func (rp RepairProgress) addRepairUnitDetailedProgress(t *table.Table, u *RepairUnitProgress) {
	for _, n := range u.Nodes {
		for i, s := range n.Shards {
			t.AddRow(n.Host, i, FormatProgress(s.PercentComplete, s.PercentFailed), s.SegmentCount, s.SegmentSuccess, s.SegmentError)
		}
	}
}

const statusDown = "DOWN"

// ClusterStatus contains cluster status info.
type ClusterStatus models.ClusterStatus

// Render renders ClusterStatus in a tabular format.
func (cs ClusterStatus) Render(w io.Writer) error {
	if len(cs) == 0 {
		return nil
	}

	var (
		dc = cs[0].Dc
		t  = table.New("CQL", "API", "SSL", "Host")
	)

	for _, s := range cs {
		if s.Dc != dc {
			if _, err := w.Write([]byte("Datacenter: " + dc + "\n" + t.String())); err != nil {
				return err
			}
			dc = s.Dc
			t = table.New("CQL", "API", "SSL", "Host")
		}
		cqlStatus := statusDown
		if s.CqlStatus != statusDown {
			cqlStatus = fmt.Sprintf("%s (%.0fms)", s.CqlStatus, s.CqlRttMs)
		}
		apiStatus := statusDown
		if s.APIStatus != statusDown {
			apiStatus = fmt.Sprintf("%s (%.0fms)", s.APIStatus, s.APIRttMs)
		}
		ssl := "OFF"
		if s.Ssl {
			ssl = "ON"
		}
		t.AddRow(cqlStatus, apiStatus, ssl, s.Host)
	}
	if _, err := w.Write([]byte("Datacenter: " + dc + "\n" + t.String())); err != nil {
		return err
	}

	return nil
}

// RepairUnitProgress contains unit progress info.
type RepairUnitProgress = models.RepairProgressUnitsItems0
