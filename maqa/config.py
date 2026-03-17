from pydantic import BaseModel, ConfigDict, model_validator


class MAQAConfig(BaseModel):
    # 配置保持不可变，避免同一个引擎实例在运行过程中出现参数漂移。
    model_config = ConfigDict(frozen=True)

    # 线性加权项。
    w_fit: float = 0.50
    w_q: float = 0.25
    w_b: float = 0.15
    w_srv: float = 0.10

    # QuotaGap 参数。
    alpha_q: float = 2.0
    epsilon_q: float = 0.2

    # Burst 参数。
    delta_b: float = 0.5
    epsilon_b: float = 0.5
    b_max: float = 2.0

    # OverQuotaDecay 参数。
    beta: float = 0.8
    eta: float = 2.0

    # 用于打破近似平分时的小扰动。
    noise_eps: float = 0.001

    @model_validator(mode="after")
    def validate_weight_sum(self) -> "MAQAConfig":
        # 当前版本要求 4 个权重之和保持为 1.0，避免总分量纲漂移。
        weight_sum = self.w_fit + self.w_q + self.w_b + self.w_srv
        if abs(weight_sum - 1.0) > 1e-9:
            raise ValueError("w_fit + w_q + w_b + w_srv must equal 1.0")
        return self
