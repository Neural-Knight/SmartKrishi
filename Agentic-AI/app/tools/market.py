import os, requests, logging
log = logging.getLogger("Market")

def market_api(crop: str, region: str="national") -> dict:
    key   = os.getenv("DATA_GOV_KEY")
    resid = os.getenv("AGMARKNET_ID")
    if not (key and resid):
        return {"crop": crop, "latest": None,
                "history": [], "source":"fallback"}
    url = f"https://api.data.gov.in/resource/{resid}"
    filt = {"filters[commodity]": crop}
    if region != "national":
        filt["filters[state]"] = region
    try:
        r = requests.get(url, params={"api-key":key,
                                      "format":"json",
                                      "limit":20,
                                      **filt})
        r.raise_for_status()
        recs   = r.json().get("records", [])
        prices = [float(rec["modal_price"]) for rec in recs
                  if rec.get("modal_price")]
        return {"crop": crop,
                "latest": prices[-1] if prices else None,
                "history": prices[-10:],
                "source": "agmarknet"}
    except Exception as e:
        log.warning("Market API error: %s", e)
        return {"crop": crop, "latest": None,
                "history": [], "source":"fallback"}
