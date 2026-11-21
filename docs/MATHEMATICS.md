# Mathematical Formulation of Long-Context Attention Degradation

## 1. Context Depth Parameterization

Let context length $L \in \{L_1, L_2, \dots, L_m\}$ denote the total token count in the evaluation prompt, and let depth $d \in [0.0, 1.0]$ denote the relative insertion coordinate of the target needle fact.

The token index offset within the document context is defined as:

$$\text{Index}(d, L) = \lfloor d \cdot L \rfloor$$

When $d = 0.0$, the needle is placed immediately following prompt instructions (primacy). When $d = 1.0$, the needle is placed immediately preceding the user query (recency).

---

## 2. Empirical Recall Function

For a trial conducted at coordinate $(L, d)$, let $y_{L, d}^{(k)} \in \{0, 1\}$ represent the binary retrieval success for trial $k \in \{1, \dots, K\}$:

$$R(L, d) = \frac{1}{K} \sum_{k=1}^K y_{L, d}^{(k)}$$

The 2D Degradation Matrix $\mathbf{M} \in \mathbb{R}^{m \times n}$ stores the recall values across sampled context lengths $L_i$ and depths $d_j$:

$$\mathbf{M}_{i, j} = R(L_i, d_j)$$

---

## 3. Parabolic U-Curve Attenuation Model

Empirical transformer attention exhibits a characteristic bowl shape across large token spans, with higher recall at the boundaries and reduced recall in the center.

### 3.1 Quadratic Regression
We model this degradation using a second-order polynomial centered at $d = 0.5$:

$$R_{\text{fit}}(d) = \alpha (d - 0.5)^2 + \gamma$$

Where:
- $\alpha > 0$ represents the curvature coefficient. A higher $\alpha$ indicates steeper attention drop-off in the middle.
- $\gamma \in [0, 1]$ represents the vertex recall at depth $d = 0.5$.
- $\beta = 1.0 - \gamma$ represents the peak central attenuation.

Using the substitution $x_i = (d_i - 0.5)^2$, the parameters are solved via ordinary least squares:

$$\alpha = \frac{\sum_{i=1}^n (x_i - \bar{x})(R_i - \bar{R})}{\sum_{i=1}^n (x_i - \bar{x})^2}$$

$$\gamma = \bar{R} - \alpha \bar{x}$$

The goodness of fit is evaluated using the coefficient of determination:

$$R^2 = 1 - \frac{\sum_{i=1}^n (R_i - R_{\text{fit}}(d_i))^2}{\sum_{i=1}^n (R_i - \bar{R})^2}$$

### 3.2 Gaussian Dip Formulation
Alternatively, the attention dip can be modeled as an inverted Gaussian:

$$R(d) = 1.0 - \beta \cdot \exp\left( -\frac{(d - 0.5)^2}{2 \sigma_{\text{mid}}^2} \right)$$

Where $\sigma_{\text{mid}}$ characterizes the width of the blind spot.

---

## 4. Multi-Hop Dependency Formulation

For multi-hop reasoning requiring $H$ distinct factual linkages at depths $\{d_1, d_2, \dots, d_H\}$, the overall success probability depends on individual token recalls and the distance between linked facts:

$$P(\text{Hop Success}) = \prod_{h=1}^H R(L, d_h) \cdot \prod_{h=2}^H \Phi_{\text{link}}(|d_h - d_{h-1}|)$$

Where the cross-attention linkage penalty is defined as:

$$\Phi_{\text{link}}(\Delta d) = \exp(-\lambda_{\text{hop}} \cdot \Delta d)$$

Here, $\lambda_{\text{hop}} > 0$ parameterizes the model's sensitivity to distance between linked facts. When two dependent facts are separated across the central attention valley, recall drops exponentially faster than single-needle retrieval.

---

## 5. Salience Re-Arrangement Strategy

### 5.1 BM25 Chunk Scoring
Given $M$ document chunks $\{C_1, \dots, C_M\}$ and user query $Q$, the salience score for chunk $C_i$ is computed using Okapi BM25:

$$\text{Score}(C_i, Q) = \sum_{q \in Q} \text{IDF}(q) \cdot \frac{f(q, C_i) \cdot (k_1 + 1)}{f(q, C_i) + k_1 \cdot \left(1 - b + b \cdot \frac{|C_i|}{\text{avgdl}}\right)}$$

Where $k_1 = 1.5$, $b = 0.75$, and $|C_i|$ is the chunk word count.

### 5.2 U-Shaped Distribution Mapping
Sort the chunks by descending salience: $s_{(1)} \ge s_{(2)} \ge \dots \ge s_{(M)}$.

The target position index $P(k) \in \{0, \dots, M-1\}$ for rank $k \in \{0, \dots, M-1\}$ is assigned using alternating placement:

$$P(k) = \begin{cases}
(M - 1) - \lfloor k / 2 \rfloor & \text{if } k \text{ is even (Recency)} \\
\lfloor k / 2 \rfloor & \text{if } k \text{ is odd (Primacy)}
\end{cases}$$

This mapping ensures:
- Rank 0 (highest salience) occupies the final slot ($M - 1$).
- Rank 1 occupies the first slot ($0$).
- Rank 2 occupies the penultimate slot ($M - 2$).
- The lowest ranked background chunks settle in the center ($\approx M / 2$).

By relocating high-salience chunks from the middle depth range ($0.35 \le d \le 0.65$) to the primacy and recency boundaries, retrieval recall improves from baseline levels ($\sim 50\%$) to boundary levels ($> 95\%$).
