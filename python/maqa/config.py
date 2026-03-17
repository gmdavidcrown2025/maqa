from pydantic import BaseModel, ConfigDict, model_validator


class MAQAConfig(BaseModel):
    # Keep configuration immutable so a single engine instance cannot drift at runtime.
    model_config = ConfigDict(frozen=True)

    # Linear weight terms.
    w_fit: float = 0.50
    w_q: float = 0.25
    w_b: float = 0.15
    w_srv: float = 0.10

    # QuotaGap parameters.
    alpha_q: float = 2.0
    epsilon_q: float = 0.2

    # Burst parameters.
    delta_b: float = 0.5
    epsilon_b: float = 0.5
    b_max: float = 2.0

    # OverQuotaDecay parameters.
    beta: float = 0.8
    eta: float = 2.0

    # Small perturbation used to break near-ties.
    noise_eps: float = 0.001

    @model_validator(mode="after")
    def validate_weight_sum(self) -> "MAQAConfig":
        # Require the four weights to sum to 1.0 to keep the score scale stable.
        weight_sum = self.w_fit + self.w_q + self.w_b + self.w_srv
        if abs(weight_sum - 1.0) > 1e-9:
            raise ValueError("w_fit + w_q + w_b + w_srv must equal 1.0")
        return self
