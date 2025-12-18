from enum import Enum


class TopicStatus(Enum):
    CREATED = 1
    ACTIVATED = 2
    RESOLVING = 3
    RESOLVED = 4
    FAILED = 5
    DELETED = 6

class TopicType(Enum):
    """Topic type for market queries
    
    Note: 
        - ALL (2): All Markets (default)
        - CATEGORICAL (1): Categorical Market
        - BINARY (0): Binary Market
    """
    ALL = 2  # All Markets - DEFAULT
    CATEGORICAL = 1  # Categorical Market
    BINARY = 0  # Binary Market

class TopicStatusFilter(Enum):
    ALL = None
    ACTIVATED = "activated"
    RESOLVED = "resolved"

class TopicSortType(Enum):
    """Sort type for market queries

    Note: There is no "unsorted" option. If sort_by is not specified,
    the API defaults to BY_TIME_DESC (newest markets first).
    """
    NO_SORT = 0  # Deprecated: API will default to BY_TIME_DESC
    BY_TIME_DESC = 1  # Sort by creation time (newest first) - DEFAULT
    BY_CUTOFF_TIME_ASC = 2  # Sort by cutoff time (ending soonest first)
    BY_VOLUME_DESC = 3  # Sort by total volume (highest first)
    BY_VOLUME_ASC = 4  # Sort by total volume (lowest first)
    BY_VOLUME_24H_DESC = 5  # Sort by 24h volume (highest first)
    BY_VOLUME_24H_ASC = 6  # Sort by 24h volume (lowest first)
    BY_VOLUME_7D_DESC = 7  # Sort by 7-day volume (highest first)
    BY_VOLUME_7D_ASC = 8  # Sort by 7-day volume (lowest first)
