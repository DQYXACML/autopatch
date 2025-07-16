package serializers

import (
	"context"
	"fmt"
	"math/big"
	"reflect"

	"github.com/jackc/pgtype"
	"gorm.io/gorm/schema"
)

var (
	big10              = big.NewInt(10)
	u256BigIntOverflow = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)
)

type U256Serializer struct{}

func init() {
	schema.RegisterSerializer("u256", U256Serializer{})
}

func (U256Serializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue interface{}) error {
	if dbValue == nil {
		return nil
	} else if field.FieldType != reflect.TypeOf((*big.Int)(nil)) {
		return fmt.Errorf("can only deserialize into a *big.Int: %T", field.FieldType)
	}

	var bigInt *big.Int

	switch v := dbValue.(type) {
	case string:
		// 直接从字符串解析
		var ok bool
		bigInt, ok = new(big.Int).SetString(v, 10)
		if !ok {
			return fmt.Errorf("failed to parse string as big.Int: %s", v)
		}
	case []byte:
		// 从字节数组解析
		var ok bool
		bigInt, ok = new(big.Int).SetString(string(v), 10)
		if !ok {
			return fmt.Errorf("failed to parse bytes as big.Int: %s", string(v))
		}
	default:
		// 尝试使用pgtype.Numeric解析（兼容性）
		numeric := new(pgtype.Numeric)
		err := numeric.Scan(dbValue)
		if err != nil {
			return fmt.Errorf("failed to scan value as numeric: %v", err)
		}

		bigInt = numeric.Int
		if numeric.Exp > 0 {
			factor := new(big.Int).Exp(big10, big.NewInt(int64(numeric.Exp)), nil)
			bigInt.Mul(bigInt, factor)
		}
	}

	if bigInt.Cmp(u256BigIntOverflow) >= 0 {
		return fmt.Errorf("deserialized number larger than u256 can hold: %s", bigInt)
	}

	field.ReflectValueOf(ctx, dst).Set(reflect.ValueOf(bigInt))
	return nil
}

func (U256Serializer) Value(ctx context.Context, field *schema.Field, dst reflect.Value, fieldValue interface{}) (interface{}, error) {
	if fieldValue == nil || (field.FieldType.Kind() == reflect.Pointer && reflect.ValueOf(fieldValue).IsNil()) {
		return nil, nil
	} else if field.FieldType != reflect.TypeOf((*big.Int)(nil)) {
		return nil, fmt.Errorf("can only serialize a *big.Int: %T", field.FieldType)
	}

	bigIntValue := fieldValue.(*big.Int)
	if bigIntValue == nil {
		return nil, nil
	}

	// 验证值在uint256范围内
	if bigIntValue.Sign() < 0 {
		return nil, fmt.Errorf("cannot serialize negative big.Int as u256: %s", bigIntValue)
	}

	if bigIntValue.Cmp(u256BigIntOverflow) >= 0 {
		return nil, fmt.Errorf("cannot serialize big.Int larger than u256: %s", bigIntValue)
	}

	// 直接返回big.Int的字符串表示，避免科学计数法
	return bigIntValue.String(), nil
}
