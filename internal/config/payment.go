package config

import (
	"gitlab.com/distributed_lab/figure"
	"gitlab.com/distributed_lab/kit/kv"
	"gitlab.com/distributed_lab/logan/v3/errors"
)

type PaymentConfig struct {
	TargetAddress string `fig:"target_address"`
}

func (c *config) PaymentConfig() PaymentConfig {
	c.once.Do(func() interface{} {
		var result PaymentConfig

		err := figure.
			Out(&result).
			With(figure.BaseHooks).
			From(kv.MustGetStringMap(c.getter, "payment")).
			Please()
		if err != nil {
			panic(errors.Wrap(err, "failed to figure out stellar"))
		}

		c.paymentConfig = result
		return nil
	})

	return c.paymentConfig
}
