package session

const (
	GrpcCorrelationIdHeader = "correlation-id"
	GrpcUserAgent           = "request-user-agent"
	GrpcUserLanguage        = "accept-language"
	GrpcSourceIp            = "source-ip"

	HttpCorrelationIdHeader = "X-Correlation-Id"
	HttpUserAgentHeader     = "User-Agent"
	HttpUserLanguageHeader  = "Accept-Language"
	HttpSourceIpHeader      = "X-Real-Ip"

	AmqpUserAgentHeader    = "User-Agent"
	AmqpUserLanguageHeader = "Accept-Language"
	AmqpSourceIpHeader     = "Source-Ip"
)
