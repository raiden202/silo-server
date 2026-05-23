package taskmanager

type Observer interface {
	TaskUpdated(TaskInfo)
}
