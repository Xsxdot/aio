package scheduler

import (
	"container/heap"
	"sync"
	"time"
)

// TaskHeap 任务堆，按照下次执行时间排序
type TaskHeap struct {
	mu    sync.RWMutex
	tasks []Task
}

// NewTaskHeap 创建新的任务堆
func NewTaskHeap() *TaskHeap {
	th := &TaskHeap{
		tasks: make([]Task, 0),
	}
	heap.Init(th)
	return th
}

// Len 返回堆中任务数量
func (th *TaskHeap) Len() int {
	return len(th.tasks)
}

// Less 比较两个任务的执行时间
func (th *TaskHeap) Less(i, j int) bool {
	return th.tasks[i].GetNextTime().Before(th.tasks[j].GetNextTime())
}

// Swap 交换两个任务位置
func (th *TaskHeap) Swap(i, j int) {
	th.tasks[i], th.tasks[j] = th.tasks[j], th.tasks[i]
}

// Push 向堆中添加任务
func (th *TaskHeap) Push(x interface{}) {
	th.tasks = append(th.tasks, x.(Task))
}

// Pop 从堆中弹出任务
func (th *TaskHeap) Pop() interface{} {
	old := th.tasks
	n := len(old)
	task := old[n-1]
	th.tasks = old[0 : n-1]
	return task
}

// SafePush 线程安全地添加任务
func (th *TaskHeap) SafePush(task Task) {
	th.mu.Lock()
	defer th.mu.Unlock()
	heap.Push(th, task)
}

// SafePop 线程安全地弹出任务
func (th *TaskHeap) SafePop() Task {
	th.mu.Lock()
	defer th.mu.Unlock()

	if th.Len() == 0 {
		return nil
	}
	return heap.Pop(th).(Task)
}

// SafePeek 线程安全地查看堆顶任务（不移除）
func (th *TaskHeap) SafePeek() Task {
	th.mu.RLock()
	defer th.mu.RUnlock()

	if th.Len() == 0 {
		return nil
	}
	return th.tasks[0]
}

// SafeRemove 线程安全地移除指定任务
func (th *TaskHeap) SafeRemove(taskID string) bool {
	th.mu.Lock()
	defer th.mu.Unlock()

	for i, task := range th.tasks {
		if task.GetID() == taskID {
			heap.Remove(th, i)
			return true
		}
	}
	return false
}

// SafeUpdate 线程安全地更新任务并重新排序
func (th *TaskHeap) SafeUpdate(task Task) {
	th.mu.Lock()
	defer th.mu.Unlock()

	// 先尝试找到并移除旧任务
	for i, t := range th.tasks {
		if t.GetID() == task.GetID() {
			heap.Remove(th, i)
			break
		}
	}

	// 添加更新后的任务
	heap.Push(th, task)
}

// SafeList 线程安全地获取所有任务列表
func (th *TaskHeap) SafeList() []Task {
	th.mu.RLock()
	defer th.mu.RUnlock()

	result := make([]Task, len(th.tasks))
	copy(result, th.tasks)
	return result
}

// SafeSize 线程安全地获取堆大小
func (th *TaskHeap) SafeSize() int {
	th.mu.RLock()
	defer th.mu.RUnlock()
	return th.Len()
}

// GetNextExecuteTime 获取下次最早执行时间
func (th *TaskHeap) GetNextExecuteTime() *time.Time {
	th.mu.RLock()
	defer th.mu.RUnlock()

	if th.Len() == 0 {
		return nil
	}

	nextTime := th.tasks[0].GetNextTime()
	return &nextTime
}

// PopReadyTasks 弹出所有到执行时间的任务
func (th *TaskHeap) PopReadyTasks(currentTime time.Time) []Task {
	th.mu.Lock()
	defer th.mu.Unlock()

	var readyTasks []Task

	for th.Len() > 0 && th.tasks[0].CanExecute(currentTime) {
		task := heap.Pop(th).(Task)
		readyTasks = append(readyTasks, task)
	}

	return readyTasks
}

// Clear 清空堆中所有任务
func (th *TaskHeap) Clear() {
	th.mu.Lock()
	defer th.mu.Unlock()
	th.tasks = th.tasks[:0]
}
