package agent

import (
	ctx "context"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"metrics/internal/logger"
	s "metrics/internal/service"

	"github.com/pquerna/ffjson/ffjson"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"go.uber.org/zap"
)

const numOfGorutines = 3

type SelfMonitor struct {
	cond           *sync.Cond
	Address        string
	Key            string
	PollInterval   time.Duration
	ReportInterval time.Duration
	Rate           int
	finish         bool
}

func (sm *SelfMonitor) collectRuntime(wg *sync.WaitGroup) {
	memStats := runtime.MemStats{}

	for {
		sm.cond.L.Lock()
		sm.cond.Wait()
		if sm.finish {
			sm.cond.L.Unlock()
			break
		}
		runtime.ReadMemStats(&memStats)
		mets[0] = s.BuildMetric("Alloc", float64(memStats.Alloc))
		mets[1] = s.BuildMetric("BuckHashSys", float64(memStats.BuckHashSys))
		mets[2] = s.BuildMetric("Frees", float64(memStats.Frees))
		mets[3] = s.BuildMetric("GCCPUFraction", memStats.GCCPUFraction)
		mets[4] = s.BuildMetric("GCSys", float64(memStats.GCSys))
		mets[5] = s.BuildMetric("HeapAlloc", float64(memStats.HeapAlloc))
		mets[6] = s.BuildMetric("HeapIdle", float64(memStats.HeapIdle))
		mets[7] = s.BuildMetric("HeapInuse", float64(memStats.HeapInuse))
		mets[8] = s.BuildMetric("HeapObjects", float64(memStats.HeapObjects))
		mets[9] = s.BuildMetric("HeapReleased", float64(memStats.HeapReleased))
		mets[10] = s.BuildMetric("HeapSys", float64(memStats.HeapSys))
		mets[11] = s.BuildMetric("LastGC", float64(memStats.LastGC))
		mets[12] = s.BuildMetric("Lookups", float64(memStats.Lookups))
		mets[13] = s.BuildMetric("MCacheInuse", float64(memStats.MCacheInuse))
		mets[14] = s.BuildMetric("MCacheSys", float64(memStats.MCacheSys))
		mets[15] = s.BuildMetric("MSpanInuse", float64(memStats.MSpanInuse))
		mets[16] = s.BuildMetric("MSpanSys", float64(memStats.MSpanSys))
		mets[17] = s.BuildMetric("Mallocs", float64(memStats.Mallocs))
		mets[18] = s.BuildMetric("NextGC", float64(memStats.NextGC))
		mets[19] = s.BuildMetric("NumForcedGC", float64(memStats.NumForcedGC))
		mets[20] = s.BuildMetric("NumGC", float64(memStats.NumGC))
		mets[21] = s.BuildMetric("OtherSys", float64(memStats.OtherSys))
		mets[22] = s.BuildMetric("PauseTotalNs", float64(memStats.PauseTotalNs))
		mets[23] = s.BuildMetric("StackInuse", float64(memStats.StackInuse))
		mets[24] = s.BuildMetric("StackSys", float64(memStats.StackSys))
		mets[25] = s.BuildMetric("Sys", float64(memStats.Sys))
		mets[26] = s.BuildMetric("TotalAlloc", float64(memStats.TotalAlloc))
		mets[27] = s.BuildMetric("RandomValue", randVal)
		mets[28] = s.BuildMetric("PollCount", pollCount)
		sm.cond.L.Unlock()
	}
	wg.Done()
	logger.Debug("goodbye from collectRuntime")
}

func (sm *SelfMonitor) collectPs(wg *sync.WaitGroup) {
	var psMem *mem.VirtualMemoryStat
	var psCPUs []float64

	for {
		sm.cond.L.Lock()
		sm.cond.Wait()
		if sm.finish {
			sm.cond.L.Unlock()
			break
		}
		psMem, _ = mem.VirtualMemory()
		psCPUs, _ = cpu.Percent(time.Second, true)
		mets[29] = s.BuildMetric("TotalMemory", float64(psMem.Total))
		mets[30] = s.BuildMetric("FreeMemory", float64(psMem.Free))
		for i, v := range psCPUs {
			idx := i + numMemMetrics
			name := fmt.Sprintf("CPUutilization%d", i+1)
			mets[idx] = s.BuildMetric(name, v)
		}
		sm.cond.L.Unlock()
	}
	wg.Done()
	logger.Debug("goodbye from collectPs")
}

func (sm *SelfMonitor) report(cx ctx.Context, wg *sync.WaitGroup) {
	url := "http://" + sm.Address + "/updates/"
	defer wg.Done()

	dataCh := make(chan []byte, sm.Rate)
	defer close(dataCh)
	wg.Add(sm.Rate)
	for i := 0; i < sm.Rate; i++ {
		go sm.sendWorker(cx, url, dataCh, wg)
	}
	reportTick := time.NewTicker(sm.ReportInterval)
	defer reportTick.Stop()
	for {
		select {
		case <-reportTick.C:
			sm.cond.L.Lock()
			data, _ := ffjson.Marshal(mets)
			sm.cond.L.Unlock()
			dataCh <- data
		case <-cx.Done():
			logger.Debug("goodbye from report...")
			return
		}
	}
}

func (sm *SelfMonitor) Run(cx ctx.Context) {
	wg := new(sync.WaitGroup)
	defer wg.Wait()
	wg.Add(numOfGorutines)
	go sm.collectRuntime(wg)
	go sm.collectPs(wg)
	go sm.report(cx, wg)

	collectTick := time.NewTicker(sm.PollInterval)
	defer collectTick.Stop()
	for {
		select {
		case <-collectTick.C:
			sm.cond.L.Lock()
			pollCount++
			randVal = rand.Float64()
			logger.Debug("POLL", zap.Int64("pollCount", pollCount))
			sm.cond.L.Unlock()
			sm.cond.Broadcast()
		case <-cx.Done():
			sm.cond.L.Lock()
			sm.finish = true
			sm.cond.L.Unlock()
			sm.cond.Broadcast()
			logger.Debug("Stop all monitoring...")
			return
		}
	}
}
