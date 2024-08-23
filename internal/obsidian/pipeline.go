package obsidian

import "go-micro.dev/v4/logger"

const pipelineMaxJobs = 100

type deferFn func() error

type DeferErrHandler func(err error)

func (v *Vault) processPipeline() {
	for {
		select {
		case <-v.ctx.Done():
			close(v.pipeCh)
			return
		case fn := <-v.pipeCh:
			if err := fn(); err != nil {
				v.l.Logf(logger.ErrorLevel, "Run job failed: %s", err)
				if v.errHandler != nil {
					go v.errHandler(err)
				}
			}
		}
	}
}
