package market

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ListingFilter struct {
	Exchange           string
	Search             string
	MinPrice           *string
	MaxPrice           *string
	MinAsk             *string
	MaxAsk             *string
	MinBid             *string
	MaxBid             *string
	MinVolume          *int64
	MaxVolume          *int64
	SettlementDateFrom *time.Time
	SettlementDateTo   *time.Time
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type ListingDetailsRow struct {
	Listing
	ExchangeName        string
	ExchangeAcronym     string
	ExchangeMICCode     string
	ExchangePolity      string
	StockOutstanding    *int64
	StockDividendYield  *string
	FuturesContractSize *int32
	FuturesContractUnit *string
	FuturesSettlement   *time.Time
	ForexBaseCurrency   *string
	ForexQuoteCurrency  *string
	ForexExchangeRate   *string
	ForexLiquidity      *string
	OptionID            *int64
	OptionTicker        *string
	OptionType          *string
	OptionStrikePrice   *string
	OptionVolatility    *string
	OptionOpenInterest  *int32
	OptionLastPrice     *string
	OptionAsk           *string
	OptionBid           *string
	OptionVolume        *int64
	OptionSettlement    *time.Time
}

func (r *Repository) ListStockExchanges(ctx context.Context) ([]StockExchange, error) {
	rows, err := r.db.Query(ctx, `select id, exchange_name, exchange_acronym, exchange_mic_code, polity, currency, time_zone,
		open_time::text, close_time::text, pre_market_open_time::text, pre_market_close_time::text,
		post_market_open_time::text, post_market_close_time::text, is_active
		from stock_exchange order by exchange_name asc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StockExchange
	for rows.Next() {
		var item StockExchange
		if err := rows.Scan(&item.ID, &item.ExchangeName, &item.ExchangeAcronym, &item.ExchangeMICCode, &item.Polity, &item.Currency,
			&item.TimeZone, &item.OpenTime, &item.CloseTime, &item.PreMarketOpenTime, &item.PreMarketCloseTime,
			&item.PostMarketOpenTime, &item.PostMarketCloseTime, &item.IsActive); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) GetStockExchange(ctx context.Context, id int64) (*StockExchange, error) {
	var item StockExchange
	err := r.db.QueryRow(ctx, `select id, exchange_name, exchange_acronym, exchange_mic_code, polity, currency, time_zone,
		open_time::text, close_time::text, pre_market_open_time::text, pre_market_close_time::text,
		post_market_open_time::text, post_market_close_time::text, is_active
		from stock_exchange where id = $1`, id).
		Scan(&item.ID, &item.ExchangeName, &item.ExchangeAcronym, &item.ExchangeMICCode, &item.Polity, &item.Currency,
			&item.TimeZone, &item.OpenTime, &item.CloseTime, &item.PreMarketOpenTime, &item.PreMarketCloseTime,
			&item.PostMarketOpenTime, &item.PostMarketCloseTime, &item.IsActive)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) GetListing(ctx context.Context, id int64) (*Listing, error) {
	var item Listing
	err := r.db.QueryRow(ctx, `select l.id, l.security_id, l.listing_type, l.stock_exchange_id, l.ticker, l.name, l.last_refresh,
		se.exchange_mic_code, l.price::text, l.ask::text, l.bid::text, l.change::text, l.volume, se.currency
		from listing l join stock_exchange se on se.id = l.stock_exchange_id where l.id = $1`, id).
		Scan(&item.ID, &item.SecurityID, &item.ListingType, &item.StockExchangeID, &item.Ticker, &item.Name, &item.LastRefresh,
			&item.ExchangeMICCode,
			&item.Price, &item.Ask, &item.Bid, &item.Change, &item.Volume, &item.Currency)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) GetListingType(ctx context.Context, id int64) (ListingType, error) {
	var item ListingType
	err := r.db.QueryRow(ctx, `select listing_type from listing where id = $1`, id).Scan(&item)
	return item, err
}

func (r *Repository) ListListingsByType(ctx context.Context, listingType ListingType, filter ListingFilter, page, size int, sortBy, sortDirection string) ([]Listing, int64, error) {
	args := []any{listingType}
	conditions := []string{"l.listing_type = $1"}
	if filter.Exchange != "" {
		args = append(args, strings.ToLower(filter.Exchange)+"%")
		idx := len(args)
		conditions = append(conditions, fmt.Sprintf("(lower(se.exchange_name) like $%d or lower(se.exchange_acronym) like $%d or lower(se.exchange_mic_code) like $%d)", idx, idx, idx))
	}
	if filter.Search != "" {
		args = append(args, "%"+strings.ToLower(filter.Search)+"%")
		idx := len(args)
		conditions = append(conditions, fmt.Sprintf("(lower(l.ticker) like $%d or lower(l.name) like $%d)", idx, idx))
	}
	if filter.MinPrice != nil {
		args = append(args, *filter.MinPrice)
		conditions = append(conditions, fmt.Sprintf("l.price >= $%d::numeric", len(args)))
	}
	if filter.MaxPrice != nil {
		args = append(args, *filter.MaxPrice)
		conditions = append(conditions, fmt.Sprintf("l.price <= $%d::numeric", len(args)))
	}
	if filter.MinAsk != nil {
		args = append(args, *filter.MinAsk)
		conditions = append(conditions, fmt.Sprintf("l.ask >= $%d::numeric", len(args)))
	}
	if filter.MaxAsk != nil {
		args = append(args, *filter.MaxAsk)
		conditions = append(conditions, fmt.Sprintf("l.ask <= $%d::numeric", len(args)))
	}
	if filter.MinBid != nil {
		args = append(args, *filter.MinBid)
		conditions = append(conditions, fmt.Sprintf("l.bid >= $%d::numeric", len(args)))
	}
	if filter.MaxBid != nil {
		args = append(args, *filter.MaxBid)
		conditions = append(conditions, fmt.Sprintf("l.bid <= $%d::numeric", len(args)))
	}
	if filter.MinVolume != nil {
		args = append(args, *filter.MinVolume)
		conditions = append(conditions, fmt.Sprintf("l.volume >= $%d", len(args)))
	}
	if filter.MaxVolume != nil {
		args = append(args, *filter.MaxVolume)
		conditions = append(conditions, fmt.Sprintf("l.volume <= $%d", len(args)))
	}
	if listingType == ListingTypeFutures && filter.SettlementDateFrom != nil {
		args = append(args, filter.SettlementDateFrom.Format("2006-01-02"))
		conditions = append(conditions, fmt.Sprintf("fc.settlement_date >= $%d", len(args)))
	}
	if listingType == ListingTypeFutures && filter.SettlementDateTo != nil {
		args = append(args, filter.SettlementDateTo.Format("2006-01-02"))
		conditions = append(conditions, fmt.Sprintf("fc.settlement_date <= $%d", len(args)))
	}
	base := `from listing l join stock_exchange se on se.id = l.stock_exchange_id left join futures_contract fc on fc.id = l.security_id and l.listing_type = 'FUTURES'`
	where := " where " + strings.Join(conditions, " and ")
	var total int64
	if err := r.db.QueryRow(ctx, "select count(*) "+base+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	orderBy := "l.ticker asc"
	if sortBy != "" {
		dir := "asc"
		if strings.EqualFold(sortDirection, "desc") {
			dir = "desc"
		}
		switch sortBy {
		case "ticker", "name", "price", "volume", "lastRefresh":
			orderBy = "l." + sortBy + " " + dir
		}
	}
	args = append(args, size, page*size)
	rows, err := r.db.Query(ctx, `select l.id, l.security_id, l.listing_type, l.stock_exchange_id, l.ticker, l.name, l.last_refresh,
		se.exchange_mic_code, l.price::text, l.ask::text, l.bid::text, l.change::text, l.volume, se.currency, fc.settlement_date `+base+where+
		" order by "+orderBy+" limit $"+fmt.Sprint(len(args)-1)+" offset $"+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Listing
	for rows.Next() {
		var item Listing
		if err := rows.Scan(&item.ID, &item.SecurityID, &item.ListingType, &item.StockExchangeID, &item.Ticker, &item.Name,
			&item.LastRefresh, &item.ExchangeMICCode, &item.Price, &item.Ask, &item.Bid, &item.Change, &item.Volume, &item.Currency, &item.SettlementDate); err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func (r *Repository) GetListingHistory(ctx context.Context, listingID int64, from time.Time) ([]DailyPriceInfo, error) {
	rows, err := r.db.Query(ctx, `select date, price::text, ask::text, bid::text, change::text, volume
		from listing_daily_price_info where listing_id = $1 and date >= $2 order by date asc`, listingID, from.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DailyPriceInfo
	for rows.Next() {
		var item DailyPriceInfo
		if err := rows.Scan(&item.Date, &item.Price, &item.Ask, &item.Bid, &item.Change, &item.Volume); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) GetListingDetailsRow(ctx context.Context, id int64) (*ListingDetailsRow, error) {
	query := `select
		l.id, l.security_id, l.listing_type, l.stock_exchange_id, l.ticker, l.name, l.last_refresh,
		l.price::text, l.ask::text, l.bid::text, l.change::text, l.volume, se.currency,
		se.exchange_name, se.exchange_acronym, se.exchange_mic_code, se.polity,
		st.outstanding_shares, st.dividend_yield::text,
		fc.contract_size, fc.contract_unit, fc.settlement_date,
		fp.base_currency, fp.quote_currency, fp.exchange_rate::text, fp.liquidity,
		so.id, so.ticker, so.option_type, so.strike_price::text, so.implied_volatility::text, so.open_interest,
		so.last_price::text, so.ask::text, so.bid::text, so.volume, so.settlement_date
		from listing l
		join stock_exchange se on se.id = l.stock_exchange_id
		left join stock st on st.id = l.security_id and l.listing_type = 'STOCK'
		left join futures_contract fc on fc.id = l.security_id and l.listing_type = 'FUTURES'
		left join forex_pair fp on fp.id = l.security_id and l.listing_type = 'FOREX'
		left join stock_option so on so.id = l.security_id and l.listing_type = 'OPTION'
		where l.id = $1`
	var row ListingDetailsRow
	err := r.db.QueryRow(ctx, query, id).Scan(
		&row.ID, &row.SecurityID, &row.ListingType, &row.StockExchangeID, &row.Ticker, &row.Name, &row.LastRefresh,
		&row.Price, &row.Ask, &row.Bid, &row.Change, &row.Volume, &row.Currency,
		&row.ExchangeName, &row.ExchangeAcronym, &row.ExchangeMICCode, &row.ExchangePolity,
		&row.StockOutstanding, &row.StockDividendYield,
		&row.FuturesContractSize, &row.FuturesContractUnit, &row.FuturesSettlement,
		&row.ForexBaseCurrency, &row.ForexQuoteCurrency, &row.ForexExchangeRate, &row.ForexLiquidity,
		&row.OptionID, &row.OptionTicker, &row.OptionType, &row.OptionStrikePrice, &row.OptionVolatility, &row.OptionOpenInterest,
		&row.OptionLastPrice, &row.OptionAsk, &row.OptionBid, &row.OptionVolume, &row.OptionSettlement,
	)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

type OptionRow struct {
	ID                int64
	Ticker            string
	OptionType        string
	StrikePrice       string
	ImpliedVolatility string
	OpenInterest      int32
	LastPrice         string
	Ask               string
	Bid               string
	Volume            int64
	SettlementDate    time.Time
}

func (r *Repository) ListOptionsForStock(ctx context.Context, stockID int64) ([]OptionRow, error) {
	rows, err := r.db.Query(ctx, `select id, ticker, option_type, strike_price::text, implied_volatility::text, open_interest,
		last_price::text, ask::text, bid::text, volume, settlement_date
		from stock_option where stock_listing_id = $1 order by settlement_date asc, strike_price asc`, stockID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OptionRow
	for rows.Next() {
		var item OptionRow
		if err := rows.Scan(&item.ID, &item.Ticker, &item.OptionType, &item.StrikePrice, &item.ImpliedVolatility, &item.OpenInterest,
			&item.LastPrice, &item.Ask, &item.Bid, &item.Volume, &item.SettlementDate); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) UpdateListingSnapshot(ctx context.Context, listingID int64, ticker string, price, ask, bid, change string, volume int64, lastRefresh time.Time) error {
	_, err := r.db.Exec(ctx, `update listing set ticker=$2, price=$3::numeric, ask=$4::numeric, bid=$5::numeric, change=$6::numeric, volume=$7, last_refresh=$8 where id=$1`,
		listingID, ticker, price, ask, bid, change, volume, lastRefresh)
	return err
}

func (r *Repository) UpsertDailySnapshot(ctx context.Context, listingID int64, day time.Time, price, ask, bid, change string, volume int64) error {
	_, err := r.db.Exec(ctx, `insert into listing_daily_price_info (listing_id, date, price, ask, bid, change, volume)
		values ($1, $2, $3::numeric, $4::numeric, $5::numeric, $6::numeric, $7)
		on conflict (listing_id, date) do update set price=excluded.price, ask=excluded.ask, bid=excluded.bid, change=excluded.change, volume=excluded.volume`,
		listingID, day.Format("2006-01-02"), price, ask, bid, change, volume)
	return err
}

func (r *Repository) ListAllListings(ctx context.Context) ([]Listing, error) {
	rows, err := r.db.Query(ctx, `select l.id, l.security_id, l.listing_type, l.stock_exchange_id, l.ticker, l.name, l.last_refresh,
		se.exchange_mic_code, l.price::text, l.ask::text, l.bid::text, l.change::text, l.volume, se.currency
		from listing l join stock_exchange se on se.id = l.stock_exchange_id order by l.id asc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Listing
	for rows.Next() {
		var item Listing
		if err := rows.Scan(&item.ID, &item.SecurityID, &item.ListingType, &item.StockExchangeID, &item.Ticker, &item.Name,
			&item.LastRefresh, &item.ExchangeMICCode, &item.Price, &item.Ask, &item.Bid, &item.Change, &item.Volume, &item.Currency); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) UpdateForexPairRate(ctx context.Context, pairID int64, rate string) error {
	_, err := r.db.Exec(ctx, `update forex_pair set exchange_rate = $2::numeric where id=$1`, pairID, rate)
	return err
}

func (r *Repository) ListStockListings(ctx context.Context) ([]Listing, error) {
	return r.listingsByType(ctx, ListingTypeStock)
}

func (r *Repository) listingsByType(ctx context.Context, listingType ListingType) ([]Listing, error) {
	rows, err := r.db.Query(ctx, `select l.id, l.security_id, l.listing_type, l.stock_exchange_id, l.ticker, l.name, l.last_refresh,
		se.exchange_mic_code, l.price::text, l.ask::text, l.bid::text, l.change::text, l.volume, se.currency
		from listing l join stock_exchange se on se.id=l.stock_exchange_id where l.listing_type = $1 order by l.id asc`, listingType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Listing
	for rows.Next() {
		var item Listing
		if err := rows.Scan(&item.ID, &item.SecurityID, &item.ListingType, &item.StockExchangeID, &item.Ticker, &item.Name,
			&item.LastRefresh, &item.ExchangeMICCode, &item.Price, &item.Ask, &item.Bid, &item.Change, &item.Volume, &item.Currency); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) GetStockByTicker(ctx context.Context, ticker string) (*struct {
	ID              int64
	Ticker          string
	Name            string
	OutstandingShares int64
	DividendYield   string
	ListingID       int64
}, error) {
	var row struct {
		ID                int64
		Ticker            string
		Name              string
		OutstandingShares int64
		DividendYield     string
		ListingID         int64
	}
	err := r.db.QueryRow(ctx, `select st.id, st.ticker, st.name, st.outstanding_shares, st.dividend_yield::text, l.id
		from stock st join listing l on l.security_id = st.id and l.listing_type='STOCK' where upper(st.ticker)=upper($1)`, ticker).
		Scan(&row.ID, &row.Ticker, &row.Name, &row.OutstandingShares, &row.DividendYield, &row.ListingID)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repository) UpdateStockFundamentals(ctx context.Context, stockID int64, name string, shares int64, dividendYield string) error {
	_, err := r.db.Exec(ctx, `update stock set name=$2, outstanding_shares=$3, dividend_yield=$4::numeric where id=$1`, stockID, name, shares, dividendYield)
	return err
}

func (r *Repository) LatestHistoryDate(ctx context.Context) (*time.Time, error) {
	var value *time.Time
	err := r.db.QueryRow(ctx, `select max(rate_date) from exchange_rate`).Scan(&value)
	return value, err
}

func (r *Repository) GetStockExchangeByMIC(ctx context.Context, mic string) (*StockExchange, error) {
	var item StockExchange
	err := r.db.QueryRow(ctx, `select id, exchange_name, exchange_acronym, exchange_mic_code, polity, currency, time_zone,
		open_time::text, close_time::text, pre_market_open_time::text, pre_market_close_time::text,
		post_market_open_time::text, post_market_close_time::text, is_active
		from stock_exchange where upper(exchange_mic_code) = upper($1)`, mic).
		Scan(&item.ID, &item.ExchangeName, &item.ExchangeAcronym, &item.ExchangeMICCode, &item.Polity, &item.Currency,
			&item.TimeZone, &item.OpenTime, &item.CloseTime, &item.PreMarketOpenTime, &item.PreMarketCloseTime,
			&item.PostMarketOpenTime, &item.PostMarketCloseTime, &item.IsActive)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) InsertStockExchange(ctx context.Context, exchange StockExchange) error {
	_, err := r.db.Exec(ctx, `insert into stock_exchange
		(exchange_name, exchange_acronym, exchange_mic_code, polity, currency, time_zone,
		 open_time, close_time, pre_market_open_time, pre_market_close_time,
		 post_market_open_time, post_market_close_time, is_active)
		values ($1, $2, $3, $4, $5, $6, $7::time, $8::time, $9::time, $10::time, $11::time, $12::time, $13)`,
		exchange.ExchangeName, exchange.ExchangeAcronym, exchange.ExchangeMICCode, exchange.Polity, exchange.Currency, exchange.TimeZone,
		exchange.OpenTime, exchange.CloseTime, exchange.PreMarketOpenTime, exchange.PreMarketCloseTime,
		exchange.PostMarketOpenTime, exchange.PostMarketCloseTime, exchange.IsActive)
	return err
}

func (r *Repository) UpdateStockExchange(ctx context.Context, exchange StockExchange) error {
	_, err := r.db.Exec(ctx, `update stock_exchange set
		exchange_name=$2, exchange_acronym=$3, polity=$4, currency=$5, time_zone=$6,
		open_time=$7::time, close_time=$8::time, pre_market_open_time=$9::time, pre_market_close_time=$10::time,
		post_market_open_time=$11::time, post_market_close_time=$12::time, is_active=$13
		where id=$1`,
		exchange.ID, exchange.ExchangeName, exchange.ExchangeAcronym, exchange.Polity, exchange.Currency, exchange.TimeZone,
		exchange.OpenTime, exchange.CloseTime, exchange.PreMarketOpenTime, exchange.PreMarketCloseTime,
		exchange.PostMarketOpenTime, exchange.PostMarketCloseTime, exchange.IsActive)
	return err
}
