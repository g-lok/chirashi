#include "StdInclude.h"

#include "SBinary.h"

#include <cmath>




namespace RSMedia {




void NibbleizeLittleEndian(std::uint8_t dest[], const std::uint8_t source[], TRawBytePos unnibbleizedSize){
	std::uint8_t b=0;

	ASSERT(dest !=0);
	ASSERT(source !=0);

	while(unnibbleizedSize-- > 0){
		b=*source++;
		*dest++=b & 15;
		*dest++=b >> 4;
	}
}
void UnnibbleizeLittleEndian(std::uint8_t dest[], const std::uint8_t source[], TRawBytePos unnibbleizedSize){
	std::uint8_t hi=0;
	std::uint8_t lo=0;

	ASSERT(dest !=0);
	ASSERT(source !=0);

	while(unnibbleizedSize-- > 0){
		lo=*source++;
		hi=*source++;
		*dest++ = static_cast<std::uint8_t>((hi << 4) | lo);
	}
}


void NibbleizeBigEndian(std::uint8_t dest[], const std::uint8_t source[], TRawBytePos unnibbleizedSize){
	std::uint8_t b=0;

	ASSERT(dest !=0);
	ASSERT(source !=0);

	while(unnibbleizedSize-- > 0){
		b=*source++;
		*dest++=b >> 4;
		*dest++=b & 15;
	}
}
void UnnibbleizeBigEndian(std::uint8_t dest[], const std::uint8_t source[], TRawBytePos unnibbleizedSize){
	std::uint8_t hi=0;
	std::uint8_t lo=0;

	ASSERT(dest !=0);
	ASSERT(source !=0);

	while(unnibbleizedSize-- > 0){
		hi=*source++;
		lo=*source++;
		*dest++ = static_cast<std::uint8_t>((hi << 4) | lo);
	}
}



void ClearBinary(std::uint8_t binary[], TRawBytePos size){
	for(TRawBytePos c=0 ; c < size ; c++){
		binary[c]=0;
	}
}

//////////////		Booleans.

std::uint8_t PackBool(bool b)
{
	return b ? 1 : 0;
}

bool UnpackBool(std::uint8_t binary)
{
	return binary ? true : false;
}









class IEEE80{
	public:
		IEEE80();

		std::uint16_t exp;
		std::uint32_t mant[2];
};

IEEE80::IEEE80() :
	exp(0)
{
	mant[0] = 0;
	mant[1] = 0;
}

double UnpackIEEE80Big(const std::uint8_t binary[]){
	double value=0;
	IEEE80 ieee;

	ieee.exp=Unpack16BitUnsignedBig(&binary[IEEE80_IMAGE_EXP]);
	ieee.mant[0]=Unpack32BitUnsignedBig(&binary[IEEE80_IMAGE_MANTISSA1]);
	ieee.mant[1]=Unpack32BitUnsignedBig(&binary[IEEE80_IMAGE_MANTISSA2]);

	if((ieee.exp == 0) && (ieee.mant[0] == 0) && (ieee.mant[1] == 0))
	{
		return 0.0;
	}

	value = static_cast<double>(ieee.mant[1]) * std::pow(2.0, -63.0);
	value += static_cast<double>(ieee.mant[0]) * std::pow(2.0, -31.0);
	value *= std::pow(2.0, static_cast<double>(ieee.exp & 0x7FFF) - 16383.0);

	return (ieee.exp & 0x8000) ? -value : value;
}

void PackIEEE80Big(std::uint8_t binary[], double value){
	IEEE80 ieee;
	bool sign=false;

	if (value == 0.0) {
		ieee.exp = 0;
		ieee.mant[0]= 0;
		ieee.mant[1] = 0;
	}
	else{
		sign = false;
		if (value < 0.0){
			sign = true;
			value = -value;
		}

		ieee.exp = (std::uint16_t)((std::log(value) / std::log(2.0) + 16383.0));
		value *= std::pow(2.0, 31.0 + 16383.0 - static_cast<double>(ieee.exp) );
		value -= (ieee.mant[0] = (std::uint32_t)value);
		value *= std::pow(2.0, 32.0);
		ieee.mant[1] = (std::uint32_t)value;

		if (sign)
		{
			ieee.exp |= 0x8000;
		}
	}

	Pack16BitUnsignedBig(&binary[IEEE80_IMAGE_EXP],ieee.exp);
	Pack32BitUnsignedBig(&binary[IEEE80_IMAGE_MANTISSA1],ieee.mant[0]);
	Pack32BitUnsignedBig(&binary[IEEE80_IMAGE_MANTISSA2],ieee.mant[1]);
}

std::int32_t UnpackIEEE80BigToLong(const std::uint8_t binary[]){
	return static_cast<std::int32_t>(UnpackIEEE80Big(binary));
}

void PackIEEE80BigFromLong(std::uint8_t binary[], std::int32_t value){
	PackIEEE80Big(binary,value);
}









std::uint8_t GetHiNybble(std::uint8_t value){
	return static_cast<std::uint8_t>((value >> 4) & 0x0f);
}

std::uint8_t GetLoNybble(std::uint8_t value){
	return static_cast<std::uint8_t>(value & 0x0f);
}

std::uint8_t NybblesToByte(std::uint8_t hiNybble, std::uint8_t loNybble){
	ASSERT(hiNybble < 16);
	ASSERT(loNybble < 16);

	return static_cast<std::uint8_t>((hiNybble << 4) | loNybble);
}




std::uint32_t Calc32BitChecksum(const std::uint8_t data[], std::uint32_t count){
	std::uint32_t sum;

	sum=0;
	while(count-- > 0){
		sum +=*data++;
	}

	return sum;
}


std::uint32_t WordsToLong(std::uint16_t hi, std::uint16_t lo){
	return ((static_cast<std::uint32_t>(hi) << static_cast<std::uint32_t>(16)) | static_cast<std::uint32_t>(lo));
}


std::uint16_t GetLoWord(std::uint32_t l){
	return static_cast<std::uint16_t>(l & static_cast<std::uint32_t>(0x0000ffff));
}
std::uint16_t GetHiWord(std::uint32_t l){
	return static_cast<std::uint16_t>(l >> static_cast<std::uint32_t>(16));
}
std::uint32_t AssembleLong(std::uint16_t hi, std::uint16_t lo){
	return (static_cast<std::uint32_t>(hi) << static_cast<std::uint32_t>(16)) | static_cast<std::uint32_t>(lo);
}



std::uint8_t GetLoByte(std::uint16_t w){
	return static_cast<std::uint8_t>(w & static_cast<std::uint16_t>(0xff));
}
std::uint8_t GetHiByte(std::uint16_t w){
	return static_cast<std::uint8_t>(w >> static_cast<std::uint16_t>(8));
}



std::uint16_t SwapWord(std::uint16_t w){
	return static_cast<std::uint16_t>((w >> static_cast<std::uint16_t>(8)) | ((w & static_cast<std::uint16_t>(0xff)) << static_cast<std::uint16_t>(8)));
}
std::uint32_t SwapLong(std::uint32_t l){
	return BytesToLong(static_cast<std::uint8_t>(l & static_cast<std::uint32_t>(0xff))
			, static_cast<std::uint8_t>((l >> static_cast<std::uint32_t>(8)) & static_cast<std::uint32_t>(0xff))
			, static_cast<std::uint8_t>((l >> static_cast<std::uint32_t>(16)) & static_cast<std::uint32_t>(0xff))
			, static_cast<std::uint8_t>(l >> static_cast<std::uint32_t>(24)));
}
void SwapWordBytes(std::uint8_t p[]){
	std::uint8_t b;

	ASSERT(p !=NULL);

	b=p[0];
	p[0]=p[1];
	p[1]=b;
}
void SwapLongBytes(std::uint8_t p[]){
	std::uint8_t b;

	ASSERT(p !=0);

	b = p[0];
	p[0] = p[3];
	p[3] = b;

	b = p[2];
	p[2] = p[1];
	p[1] = b;
}

void SwapWordArray(std::uint16_t data[], std::uint32_t wordCount){
	ASSERT(data !=0);

	std::uint16_t* p = reinterpret_cast<std::uint16_t*>(data);
	for (std::uint32_t count=wordCount ; count-- > 0 ; ){
		std::uint16_t v = *p;
		*p++ = static_cast<std::uint16_t>((v >> 8) | (v << 8));
	}
}
















std::uint8_t* big7split(std::uint32_t value, std::int32_t byteCount, std::uint8_t buffer[])
{
	std::uint8_t* ret = (buffer += byteCount);
	while (--byteCount >= 0) {
		*--buffer = static_cast<std::uint8_t>(value & 0x7F);
		value >>= 7;
	}
	return ret;
}

std::uint8_t* big7join(std::int32_t byteCount, const std::uint8_t data[], std::uint32_t* value){
	for(std::int32_t index=0 ; index < byteCount ; index++){
		*value=(*value << 7) | (data[index] & 0x7f);
	}
	return const_cast<std::uint8_t*>(&data[byteCount]);
}

std::uint8_t* big4split(std::uint32_t value, std::int32_t byteCount, std::uint8_t buffer[])
{
	std::uint8_t* ret = (buffer += byteCount);
	while (--byteCount >= 0) {
		*--buffer = static_cast<std::uint8_t>(value & 0x0F);
		value >>= 4;
	}
	return ret;
}

std::uint8_t* big4join(std::int32_t byteCount, const std::uint8_t data[], std::uint32_t* value){
	for(std::int32_t index=0 ; index < byteCount ; index++){
		*value=(*value << 4) | (data[index] & 0x0f);
	}
	return const_cast<std::uint8_t*>(&data[byteCount]);
}

std::uint8_t* big4decode(const std::uint8_t nibbles[], std::int32_t byteCount, std::uint8_t bytes[]){
	for(std::int32_t index=0 ; index < byteCount ; index++){
		bytes[index]=static_cast<std::uint8_t>((nibbles[index * 2 + 0] << 4) | (nibbles[index * 2 + 1] & 0x0f));
	}
	return const_cast<std::uint8_t*>(&nibbles[byteCount * 2]);
}

std::uint8_t* big4encode(std::uint8_t nibbles[], std::int32_t byteCount, const std::uint8_t bytes[]){
	for(std::int32_t index=0 ; index < byteCount ; index++){
		std::uint8_t byte1=bytes[index];

		nibbles[index * 2 + 0]=static_cast<std::uint8_t>((byte1 & 0xf0) >> 4);
		nibbles[index * 2 + 1]=static_cast<std::uint8_t>(byte1 & 0x0f);
	}
	return reinterpret_cast<std::uint8_t*>(&nibbles[byteCount * 2]);
}

std::uint32_t decodeBig(std::int32_t bytes, const std::uint8_t data[])
{
	std::uint32_t val;
	
	val = 0;
	while (--bytes >= 0) {
		val <<= 8;
		val |=*data++;
	}
	return val;
}

std::uint32_t decodeLittle(std::int32_t bytes, const std::uint8_t data[])
{
	std::uint32_t val;
	
	data += bytes;
	val = 0;
	while (--bytes >= 0) {
		val <<= 8;
		val |=*--data;
	}
	return val;
}

std::uint8_t* encodeBig(std::uint32_t value, std::int32_t bytes, std::uint8_t data[])
{
	std::uint8_t* ret;
	
	data += bytes;
	ret = data;
	while (--bytes >= 0) {
		*--data = static_cast<std::uint8_t>(value & 0xFF);
		value >>= 8;
	}
	return ret;
}

std::uint8_t* encodeLittle(std::uint32_t value, std::int32_t bytes, std::uint8_t data[])
{
	while (--bytes >= 0) {
		*data++ = static_cast<std::uint8_t>(value & 0xFF);
		value >>= 8;
	}
	return data;
}




std::uint8_t BCDToNormal(std::uint8_t twoDigitBCD){
	std::uint8_t tenths;
	std::uint8_t ones;
	std::uint8_t normal;

	ones=static_cast<std::uint8_t>(twoDigitBCD & 0x0f);
	ASSERT(ones < 10);

	tenths=static_cast<std::uint8_t>(twoDigitBCD >> 4);
	ASSERT(tenths < 10);

	normal=static_cast<std::uint8_t>(tenths * 10 + ones);

	ASSERT(normal < 100);
	return normal;
}


void LongToBytes(std::uint32_t value, std::uint8_t* byte24, std::uint8_t* byte16, std::uint8_t* byte8, std::uint8_t* byte0){
	std::uint32_t tempValue=value;

	ASSERT(byte24 !=0);
	ASSERT(byte16 !=0);
	ASSERT(byte8 !=0);
	ASSERT(byte0 !=0);

	*byte0=static_cast<std::uint8_t>(tempValue);
	tempValue >>=8;
	*byte8=static_cast<std::uint8_t>(tempValue);
	tempValue >>=8;
	*byte16=static_cast<std::uint8_t>(tempValue);
	tempValue >>=8;
	*byte24=static_cast<std::uint8_t>(tempValue);

	ASSERT(BytesToLong(*byte24,*byte16,*byte8,*byte0)==value);
}
void WordToBytes(std::uint16_t value, std::uint8_t* byte8, std::uint8_t* byte0){
	ASSERT(byte8 !=0);
	ASSERT(byte0 !=0);

	*byte0=static_cast<std::uint8_t>(value);
	value >>=8;
	*byte8=static_cast<std::uint8_t>(value);

	ASSERT(BytesToWord(*byte8,*byte0)==value);
}
std::uint32_t BytesToLong(std::uint8_t byte24, std::uint8_t byte16, std::uint8_t byte8, std::uint8_t byte0){
	return (static_cast<std::uint32_t>(byte24) << static_cast<std::uint32_t>(24))
			| (static_cast<std::uint32_t>(byte16) << static_cast<std::uint32_t>(16))
			| (static_cast<std::uint32_t>(byte8) << static_cast<std::uint32_t>(8))
			| (static_cast<std::uint32_t>(byte0));
}
std::uint16_t BytesToWord(std::uint8_t byte8, std::uint8_t byte0){
	return static_cast<std::uint16_t>((static_cast<std::uint32_t>(byte8) << static_cast<std::uint32_t>(8)) | static_cast<std::uint32_t>(byte0));
}




std::uint8_t GetByte0(std::uint32_t value){
	return static_cast<std::uint8_t>(value & 0xff);
}
std::uint8_t GetByte8(std::uint32_t value){
	return static_cast<std::uint8_t>((value >> static_cast<std::uint32_t>(8)) & 0xff);
}
std::uint8_t GetByte16(std::uint32_t value){
	return static_cast<std::uint8_t>((value >> static_cast<std::uint32_t>(16)) & 0xff);
}
std::uint8_t GetByte24(std::uint32_t value){
	return static_cast<std::uint8_t>((value >> static_cast<std::uint32_t>(24)) & 0xff);
}



void ClearByteArray(std::uint8_t array[], std::uint32_t count){
	ASSERT(array !=NULL);

	std::memset(array,0,count * sizeof(std::uint8_t));
}


std::uint32_t BitToValue(std::uint8_t bitNumber){
	return static_cast<std::uint32_t>(1) << static_cast<std::uint32_t>(bitNumber);
}

bool IsBitSet(std::uint32_t v, std::uint8_t bitNumber){
	return v & BitToValue(bitNumber) ? true : false;
}

bool IsBinary(std::uint32_t v){
	return CountBits(v) <=1 ? true : false;
}

std::uint8_t CountBits(std::uint32_t v){
	return CountBitsInRange(v,0,31);
}

std::uint8_t CountBitsInRange(std::uint32_t v, std::uint8_t startBit, std::uint8_t endBit){
	std::uint8_t count=0;
	for(std::uint8_t bitIndex=startBit ; bitIndex <=endBit ; bitIndex++){
		if(IsBitSet(v,bitIndex)){
			count++;
		}
	}
	return count;
}



std::uint32_t PackBits(const bool bitArray[], std::uint16_t count){
	std::uint32_t packed;

	ASSERT(bitArray !=NULL);
	ASSERT(count <=32);

	packed=0;
	for(std::uint16_t bit=0 ; bit < count ; bit++){
		if(bitArray[bit]){
			packed |=1 << bit;
		}
	}
	return packed;
}

void UnpackBits(std::uint32_t packed, bool bitArray[], std::uint16_t count){
	ASSERT(bitArray !=NULL);
	ASSERT(count <=32);

	for(std::uint16_t bit=0 ; bit < count ; bit++){
		if(packed & (1 << bit))	{
			bitArray[bit]=true;
		} else {
			bitArray[bit]=false;
		}
	}
}






std::uint8_t PackByte(bool bit7, bool bit6, bool bit5, bool bit4, bool bit3, bool bit2, bool bit1, bool bit0){
	return static_cast<std::uint8_t>(( (bit7 ? 1 : 0) << 7)
		| ( (bit6 ? 1 : 0) << 6)
		| ( (bit5 ? 1 : 0) << 5)
		| ( (bit4 ? 1 : 0) << 4)
		| ( (bit3 ? 1 : 0) << 3)
		| ( (bit2 ? 1 : 0) << 2)
		| ( (bit1 ? 1 : 0) << 1)
		| ( (bit0 ? 1 : 0) << 0));
}




bool FindByteOfTypeInArray(std::uint8_t array[], std::int32_t count, std::uint8_t b){
	ASSERT(array !=NULL);
	ASSERT(count >=0);

	for(std::uint16_t index=0 ; index < count ; index++){
		if(array[index]==b){
			return true;
		}
	}
	return false;
}









///////////////////		Longs to 64bit and vice versa.







std::uint64_t LongsToUInt64(std::uint32_t hi, std::uint32_t low){
	std::uint64_t uint64=(static_cast<std::uint64_t>(hi) << 32) | low;

#if DEBUG
	//	Make sure we can convert back=conversion isn't losing some stuff.
	std::uint32_t testHi=0;
	std::uint32_t testLow=0;
	UInt64ToLongs(uint64,testHi,testLow);
	ASSERT(testHi==hi);
	ASSERT(testLow==low);
#endif	//	DEBUG

	return uint64;
}

void UInt64ToLongs(std::uint64_t uint64, std::uint32_t& hi, std::uint32_t& low){
	hi=static_cast<std::uint32_t>(uint64 >> 32);
	low=static_cast<std::uint32_t>(uint64 & static_cast<std::uint32_t>(0xffffffff));

#if DEBUG
	//	Make sure we can convert back=conversion isn't losing some stuff.
	std::uint64_t test64=(static_cast<std::uint64_t>(hi) << 32) | low;
	ASSERT(test64==uint64);
#endif	//	DEBUG
}



}	//	RSMedia

