package config

import (
	"gitlab.com/distributed_lab/figure"
	"gitlab.com/distributed_lab/kit/kv"
	"gitlab.com/distributed_lab/logan/v3/errors"
	"gitlab.com/tokend/keypair"
	"gitlab.com/tokend/keypair/figurekeypair"
	"time"
)

type DepositConfig struct {
	AssetOwner keypair.Address `fig:"asset_owner"`
	AssetIssuer keypair.Full `fig:"asset_issuer"`
	Delay time.Duration `fig:"delay"`
}
func (c *config) DepositConfig() DepositConfig {
	c.once.Do(func() interface{} {
		var result DepositConfig

		err := figure.
			Out(&result).
			With(figure.BaseHooks, figurekeypair.Hooks).
			From(kv.MustGetStringMap(c.getter, "deposit")).
			Please()
		if err != nil {
			panic(errors.Wrap(err, "failed to figure out deposit"))
		}

		c.depositConfig = result
		return nil
	})

	return c.depositConfig
}