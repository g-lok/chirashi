#pragma once

#include "SmugglerTypes.h"

#include "Core/Bacteria/Bacteria.h"
#include "Core/Debug/CheckedCast.h"
#include "Core/Text/TextPackage.h"

#include <string>
#include <math.h>


namespace RSMedia {



static const std::int32_t kMin24BitValue = -8388608;
static const std::int32_t kMax24BitValue = 8388607;

typedef std::uint32_t TMagicNumber;


//	Same as std::memset(), only easier to use.
void ClearBinary(std::uint8_t binary[], TRawBytePos size);


////////////////	Booleans.

std::uint8_t PackBool(bool b);
bool UnpackBool(std::uint8_t binary);



///////////////		Integers in both little and big endian.

//	Little endian binaries = Intel.
inline std::uint16_t Unpack16BitUnsignedLittle(const std::uint8_t binary[]);
inline void Pack16BitUnsignedLittle(std::uint8_t binary[], std::uint16_t v);

inline std::uint32_t Unpack24BitUnsignedLittle(const std::uint8_t binary[]);
inline void Pack24BitUnsignedLittle(std::uint8_t binary[], std::uint32_t v);

inline std::int32_t Unpack24BitSignedLittle(const std::uint8_t binary[]);
inline void Pack24BitSignedLittle(std::uint8_t binary[], std::int32_t v);

inline std::uint32_t Unpack32BitUnsignedLittle(const std::uint8_t binary[]);
inline void Pack32BitUnsignedLittle(std::uint8_t binary[], std::uint32_t v);

inline std::uint64_t Unpack64BitUnsignedLittle(const std::uint8_t binary[]);
inline void Pack64BitUnsignedLittle(std::uint8_t binary[], std::uint64_t v);


//	Big endian binaries= Motorola.
inline std::uint16_t Unpack16BitUnsignedBig(const std::uint8_t binary[]);
inline void Pack16BitUnsignedBig(std::uint8_t binary[], std::uint16_t v);

inline std::uint32_t Unpack24BitUnsignedBig(const std::uint8_t binary[]);
inline void Pack24BitUnsignedBig(std::uint8_t binary[], std::uint32_t v);

inline std::int32_t Unpack24BitSignedBig(const std::uint8_t binary[]);
inline void Pack24BitSignedBig(std::uint8_t binary[], std::int32_t v);

inline std::uint32_t Unpack32BitUnsignedBig(const std::uint8_t binary[]);
inline void Pack32BitUnsignedBig(std::uint8_t binary[], std::uint32_t v);

inline std::uint64_t Unpack64BitUnsignedBig(const std::uint8_t binary[]);
inline void Pack64BitUnsignedBig(std::uint8_t binary[], std::uint64_t v);


// Native endian binaries
#if IS_MOTOROLA_TO_NATIVE_A_SWAP

#define Unpack16BitUnsignedNative(binary) Unpack16BitUnsignedLittle(binary)
#define Pack16BitUnsignedNative(binary, v) Pack16BitUnsignedLittle(binary, v)

#define Unpack24BitUnsignedNative(binary) Unpack24BitUnsignedLittle(binary)
#define Pack24BitUnsignedNative(binary, v) Pack24BitUnsignedLittle(binary, v)

#define Unpack24BitSignedNative(binary) Unpack24BitSignedLittle(binary)
#define Pack24BitSignedNative(binary, v) Pack24BitSignedLittle(binary, v)

#define Unpack32BitUnsignedNative(binary) Unpack32BitUnsignedLittle(binary)
#define Pack32BitUnsignedNative(binary, v) Pack32BitUnsignedLittle(binary, v)

#define Unpack64BitUnsignedNative(binary) Unpack64BitUnsignedLittle(binary)
#define Pack64BitUnsignedNative(binary, v) Pack64BitUnsignedLittle(binary, v)

#else // IS_MOTOROLA_TO_NATIVE_A_SWAP

#define Unpack16BitUnsignedNative(binary) Unpack16BitUnsignedBig(binary)
#define Pack16BitUnsignedNative(binary, v) Pack16BitUnsignedBig(binary, v)

#define Unpack24BitUnsignedNative(binary) Unpack24BitUnsignedBig(binary)
#define Pack24BitUnsignedNative(binary, v) Pack24BitUnsignedBig(binary, v)

#define Unpack24BitSignedNative(binary) Unpack24BitSignedBig(binary)
#define Pack24BitSignedNative(binary, v) Pack24BitSignedBig(binary, v)

#define Unpack32BitUnsignedNative(binary) Unpack32BitUnsignedBig(binary)
#define Pack32BitUnsignedNative(binary, v) Pack32BitUnsignedBig(binary, v)

#define Unpack64BitUnsignedNative(binary) Unpack64BitUnsignedBig(binary)
#define Pack64BitUnsignedNative(binary, v) Pack64BitUnsignedBig(binary, v)

#endif // IS_MOTOROLA_TO_NATIVE_A_SWAP


///////////		IEEE80

typedef enum {
	IEEE80_IMAGE_EXP		=	0,
	IEEE80_IMAGE_MANTISSA1	=	2,
	IEEE80_IMAGE_MANTISSA2	=	6,
	IEEE80_IMAGE_SIZE		=	10
} IEEE80_IMAGE;

double UnpackIEEE80Big(const std::uint8_t binary[]);
void PackIEEE80Big(std::uint8_t binary[], double value);

std::int32_t UnpackIEEE80BigToLong(const std::uint8_t binary[]);
void PackIEEE80BigFromLong(std::uint8_t binary[], std::int32_t value);




// ### FL: Many of these functions are unused
//	and there are copies of them in some packages that were dlls before. Fuck.


////////////	Nybbles.

void NibbleizeLittleEndian(std::uint8_t dest[], const std::uint8_t source[], TRawBytePos unnibbleizedSize);
void UnnibbleizeLittleEndian(std::uint8_t dest[], const std::uint8_t source[], TRawBytePos unnibbleizedSize);

void NibbleizeBigEndian(std::uint8_t dest[], const std::uint8_t source[], TRawBytePos unnibbleizedSize);
void UnnibbleizeBigEndian(std::uint8_t dest[], const std::uint8_t source[], TRawBytePos unnibbleizedSize);

std::uint8_t GetHiNybble(std::uint8_t value);
std::uint8_t GetLoNybble(std::uint8_t value);
std::uint8_t NybblesToByte(std::uint8_t hiNybble, std::uint8_t loNybble);



//	Bytes/words and longs.
std::uint32_t WordsToLong(std::uint16_t hi, std::uint16_t lo);

std::uint16_t GetLoWord(std::uint32_t l);
std::uint16_t GetHiWord(std::uint32_t l);
std::uint32_t AssembleLong(std::uint16_t hi, std::uint16_t lo);

std::uint8_t GetLoByte(std::uint16_t w);
std::uint8_t GetHiByte(std::uint16_t w);

void LongToBytes(std::uint32_t value, std::uint8_t* byte24, std::uint8_t* byte16, std::uint8_t* byte8, std::uint8_t* byte0);
void WordToBytes(std::uint16_t value, std::uint8_t* byte8, std::uint8_t* byte0);
std::uint32_t BytesToLong(std::uint8_t byte24, std::uint8_t byte16, std::uint8_t byte8, std::uint8_t byte0);
std::uint16_t BytesToWord(std::uint8_t byte8, std::uint8_t byte0);

std::uint8_t GetByte0(std::uint32_t value);
std::uint8_t GetByte8(std::uint32_t value);
std::uint8_t GetByte16(std::uint32_t value);
std::uint8_t GetByte24(std::uint32_t value);


//	Swapping. Should rarely be needed. Use Binary.h to unpack/pack with endianess.
std::uint16_t SwapWord(std::uint16_t w);
std::uint32_t SwapLong(std::uint32_t l);
void SwapWordBytes(std::uint8_t p[]);
void SwapLongBytes(std::uint8_t p[]);
void SwapWordArray(std::uint16_t data[], std::uint32_t wordCount);


//	BCD.
std::uint8_t BCDToNormal(std::uint8_t twoDigitBCD);


std::uint8_t* big7split(std::uint32_t value, std::int32_t byteCount, std::uint8_t buffer[]);
std::uint8_t* big7join(std::int32_t byteCount, const std::uint8_t data[], std::uint32_t* value);
std::uint8_t* big4split(std::uint32_t value, std::int32_t byteCount, std::uint8_t buffer[]);
std::uint8_t* big4join(std::int32_t byteCount, const std::uint8_t data[], std::uint32_t* value);
std::uint8_t* big4decode(const std::uint8_t nibbles[], std::int32_t byteCount, std::uint8_t bytes[]);
std::uint8_t* big4encode(std::uint8_t nibbles[], std::int32_t byteCount, const std::uint8_t bytes[]);
std::uint32_t decodeBig(std::int32_t bytes, const std::uint8_t data[]);
std::uint32_t decodeLittle(std::int32_t bytes, const std::uint8_t data[]);
std::uint8_t* encodeBig(std::uint32_t value, std::int32_t bytes, std::uint8_t data[]);
std::uint8_t* encodeLittle(std::uint32_t value, std::int32_t bytes, std::uint8_t data[]);


//	Byte arrays.
std::uint32_t Calc32BitChecksum(const std::uint8_t data[], std::uint32_t count);
bool FindByteOfTypeInArray(std::uint8_t array[], std::int32_t count, std::uint8_t b);
void ClearByteArray(std::uint8_t array[], std::uint32_t count);


//	Bits.
std::uint32_t BitToValue(std::uint8_t bitNumber);
bool IsBitSet(std::uint32_t v, std::uint8_t bitNumber);
bool IsBinary(std::uint32_t v);
std::uint8_t CountBits(std::uint32_t v);
std::uint8_t CountBitsInRange(std::uint32_t v, std::uint8_t startBit, std::uint8_t endBit);

std::uint32_t PackBits(const bool bitArray[], std::uint16_t count);
void UnpackBits(std::uint32_t packed, bool bitArray[], std::uint16_t count);

std::uint8_t PackByte(bool bit7, bool bit6, bool bit5, bool bit4, bool bit3, bool bit2, bool bit1, bool bit0);





///////////////////		Longs to 64bit and vice versa.

std::uint64_t LongsToUInt64(std::uint32_t hi, std::uint32_t low);
void UInt64ToLongs(std::uint64_t uint64, std::uint32_t& hi, std::uint32_t& low);





/////////////////////////////		Optimized floating point conversions.

#if WINDOWS
inline std::int32_t RoundFloatToInt32(float iValue){
	ASSERT(iValue >= -2147483648.0);
	ASSERT(iValue <= 2147483647.0);

	return (iValue >= 0) ? static_cast<std::int32_t>(iValue + 0.5) : static_cast<std::int32_t>(iValue - 0.5);
}
#endif // WINDOWS

#if MAC
// PF: lrintf() is C99, will use current rounding mode so it does not need to setup FPU control register.
inline std::int32_t RoundFloatToInt32(float iValue){
	ASSERT(iValue >= -2147483648.0f);
	ASSERT(iValue <= 2147483647.0f);
	return CheckedCast<std::int32_t>(lrintf(iValue));
}
#endif // MAC

inline std::int32_t TruncFloatToInt32(float value){
	ASSERT(value>=-2147483648.0f);
	ASSERT(value<=2147483647.0f);
	return static_cast<std::int32_t>(value);
}

static inline std::uint8_t TruncFloatToUInt8(float value){
	std::uint8_t ubyteValue = static_cast<std::uint8_t>(value);
	return ubyteValue;
}


// Optimized pack/unpack per platform

#if IS_MOTOROLA_TO_NATIVE_A_SWAP

std::uint16_t Unpack16BitUnsignedLittle(const std::uint8_t binary[]){
	return *reinterpret_cast<const std::uint16_t*>(&binary[0]);
}
void Pack16BitUnsignedLittle(std::uint8_t binary[], std::uint16_t v){
	ASSERT(binary != NULL);

	*reinterpret_cast<std::uint16_t*>(&binary[0]) = v;

	ASSERT(Unpack16BitUnsignedLittle(binary)==v);
}

std::uint32_t Unpack32BitUnsignedLittle(const std::uint8_t binary[]){
	return *reinterpret_cast<const std::uint32_t*>(&binary[0]);
}
void Pack32BitUnsignedLittle(std::uint8_t binary[], std::uint32_t v){
	ASSERT(binary != NULL);

	*reinterpret_cast<std::uint32_t*>(&binary[0]) = v;

	ASSERT(Unpack32BitUnsignedLittle(binary)==v);
}

std::uint64_t Unpack64BitUnsignedLittle(const std::uint8_t binary[]) {
	return *reinterpret_cast<const std::uint64_t*>(&binary[0]);
}

void Pack64BitUnsignedLittle(std::uint8_t binary[], std::uint64_t v) {
	ASSERT(binary != NULL);

	*reinterpret_cast<std::uint64_t*>(&binary[0]) = v;

	ASSERT(Unpack64BitUnsignedLittle(binary) == v);
}

std::uint16_t Unpack16BitUnsignedBig(const std::uint8_t binary[]){
	return static_cast<std::uint16_t>( (static_cast<std::uint16_t>(binary[0]) << 8) | static_cast<std::uint16_t>(binary[1]) );
}

void Pack16BitUnsignedBig(std::uint8_t binary[], std::uint16_t v){
	ASSERT(binary != NULL);

	binary[1]=static_cast<std::uint8_t>(v);
	binary[0]=static_cast<std::uint8_t>(v >> 8);

	ASSERT(Unpack16BitUnsignedBig(binary)==v);
}

std::uint32_t Unpack32BitUnsignedBig(const std::uint8_t binary[]){
	return (static_cast<std::uint32_t>(binary[3]) ) |
		(static_cast<std::uint32_t>(binary[2]) << 8) |
		(static_cast<std::uint32_t>(binary[1]) << 16) |
		(static_cast<std::uint32_t>(binary[0]) << 24);
}

void Pack32BitUnsignedBig(std::uint8_t binary[], std::uint32_t v){
	ASSERT(binary != NULL);

	binary[3]=static_cast<std::uint8_t>(v);
	binary[2]=static_cast<std::uint8_t>(v >> 8);
	binary[1]=static_cast<std::uint8_t>(v >> 16);
	binary[0]=static_cast<std::uint8_t>(v >> 24);

	ASSERT(Unpack32BitUnsignedBig(binary)==v);
}

std::uint64_t Unpack64BitUnsignedBig(const std::uint8_t binary[]) {
	return (static_cast<std::uint64_t>(binary[7]) ) |
		(static_cast<std::uint64_t>(binary[6]) << 8) |
		(static_cast<std::uint64_t>(binary[5]) << 16) |
		(static_cast<std::uint64_t>(binary[4]) << 24) |
		(static_cast<std::uint64_t>(binary[3]) << 32) |
		(static_cast<std::uint64_t>(binary[2]) << 40) |
		(static_cast<std::uint64_t>(binary[1]) << 48) |
		(static_cast<std::uint64_t>(binary[0]) << 56);
}

void Pack64BitUnsignedBig(std::uint8_t binary[], std::uint64_t v) {
	ASSERT(binary != NULL);

	binary[7]=static_cast<std::uint8_t>(v);
	binary[6]=static_cast<std::uint8_t>(v >> 8);
	binary[5]=static_cast<std::uint8_t>(v >> 16);
	binary[4]=static_cast<std::uint8_t>(v >> 24);
	binary[3]=static_cast<std::uint8_t>(v >> 32);
	binary[2]=static_cast<std::uint8_t>(v >> 40);
	binary[1]=static_cast<std::uint8_t>(v >> 48);
	binary[0]=static_cast<std::uint8_t>(v >> 56);

	ASSERT(Unpack64BitUnsignedBig(binary)==v);
}

#else // !IS_MOTOROLA_TO_NATIVE_A_SWAP

std::uint16_t Unpack16BitUnsignedLittle(const std::uint8_t binary[]){
	return static_cast<std::uint16_t>( static_cast<std::uint32_t>(binary[0]) | (static_cast<std::uint32_t>(binary[1]) << 8) );
}
void Pack16BitUnsignedLittle(std::uint8_t binary[], std::uint16_t v){
	ASSERT(binary != NULL);

	binary[0]=static_cast<std::uint8_t>(v);
	binary[1]=static_cast<std::uint8_t>(v >> 8);

	ASSERT(Unpack16BitUnsignedLittle(binary)==v);
}

std::uint32_t Unpack32BitUnsignedLittle(const std::uint8_t binary[]){
	return (static_cast<std::uint32_t>(binary[0])) |
		(static_cast<std::uint32_t>(binary[1]) << 8) |
		(static_cast<std::uint32_t>(binary[2]) << 16) |
		(static_cast<std::uint32_t>(binary[3]) << 24);
}
void Pack32BitUnsignedLittle(std::uint8_t binary[], std::uint32_t v){
	ASSERT(binary != NULL);

	binary[0]=static_cast<std::uint8_t>(v);
	binary[1]=static_cast<std::uint8_t>(v >> 8);
	binary[2]=static_cast<std::uint8_t>(v >> 16);
	binary[3]=static_cast<std::uint8_t>(v >> 24);

	ASSERT(Unpack32BitUnsignedLittle(binary)==v);
}

std::uint64_t Unpack64BitUnsignedLittle(const std::uint8_t binary[]) {
	return (static_cast<std::uint64_t>(binary[0])) |
		(static_cast<std::uint64_t>(binary[1]) << 8) |
		(static_cast<std::uint64_t>(binary[2]) << 16) |
		(static_cast<std::uint64_t>(binary[3]) << 24) |
		(static_cast<std::uint64_t>(binary[4]) << 32) |
		(static_cast<std::uint64_t>(binary[5]) << 40) |
		(static_cast<std::uint64_t>(binary[6]) << 48) |
		(static_cast<std::uint64_t>(binary[7]) << 56);
}

void Pack64BitUnsignedLittle(std::uint8_t binary[], std::uint64_t v) {
	ASSERT(binary != NULL);

	binary[0]=static_cast<std::uint8_t>(v);
	binary[1]=static_cast<std::uint8_t>(v >> 8);
	binary[2]=static_cast<std::uint8_t>(v >> 16);
	binary[3]=static_cast<std::uint8_t>(v >> 24);
	binary[4]=static_cast<std::uint8_t>(v >> 32);
	binary[5]=static_cast<std::uint8_t>(v >> 40);
	binary[6]=static_cast<std::uint8_t>(v >> 48);
	binary[7]=static_cast<std::uint8_t>(v >> 56);

	ASSERT(Unpack64BitUnsignedLittle(binary) == v);
}

std::uint16_t Unpack16BitUnsignedBig(const std::uint8_t binary[]){
	return *reinterpret_cast<const std::uint16_t*>(&binary[0]);
}

void Pack16BitUnsignedBig(std::uint8_t binary[], std::uint16_t v){
	ASSERT(binary != NULL);

	*reinterpret_cast<std::uint16_t*>(&binary[0]) = v;

	ASSERT(Unpack16BitUnsignedBig(binary)==v);
}

std::uint32_t Unpack32BitUnsignedBig(const std::uint8_t binary[]){
	return *reinterpret_cast<const std::uint32_t*>(&binary[0]);
}

void Pack32BitUnsignedBig(std::uint8_t binary[], std::uint32_t v){
	ASSERT(binary != NULL);

	*reinterpret_cast<std::uint32_t*>(&binary[0]) = v;

	ASSERT(Unpack32BitUnsignedBig(binary)==v);
}

std::uint64_t Unpack64BitUnsignedBig(const std::uint8_t binary[]) {
	return *reinterpret_cast<const std::uint64_t*>(&binary[0]);
}

void Pack64BitUnsignedBig(std::uint8_t binary[], std::uint64_t v) {
	ASSERT(binary != NULL);

	*reinterpret_cast<std::uint64_t*>(&binary[0]) = v;

	ASSERT(Unpack64BitUnsignedBig(binary)==v);
}

#endif // !IS_MOTOROLA_TO_NATIVE_A_SWAP

std::uint32_t Unpack24BitUnsignedLittle(const std::uint8_t binary[]){
	return (std::uint32_t)(binary[0]) |
		((std::uint32_t)binary[1] << 8) |
		((std::uint32_t)binary[2] << 16);
}
void Pack24BitUnsignedLittle(std::uint8_t binary[], std::uint32_t v){
	ASSERT(binary != NULL);

	binary[0]=static_cast<std::uint8_t>(v);
	binary[1]=static_cast<std::uint8_t>(v >> 8);
	binary[2]=static_cast<std::uint8_t>(v >> 16);

	ASSERT(Unpack24BitUnsignedLittle(binary)==v);
}

std::int32_t Unpack24BitSignedLittle(const std::uint8_t binary[]){
	ASSERT(binary!=0);
	std::uint32_t a = static_cast<std::uint32_t>(binary[0]) | (static_cast<std::uint32_t>(binary[1]) << 8);
	std::int32_t b = static_cast<std::int8_t>(binary[2]) << 16;
	a |= static_cast<std::uint32_t>(b);
	return static_cast<std::int32_t>(a);
}
void Pack24BitSignedLittle(std::uint8_t binary[], std::int32_t v){
	ASSERT(binary != NULL);
	ASSERT(v>=kMin24BitValue && v<=kMax24BitValue);
	binary[0]=static_cast<std::uint8_t>(v);
	binary[1]=static_cast<std::uint8_t>(v >> 8);
	binary[2]=static_cast<std::uint8_t>(v >> 16);
	ASSERT(Unpack24BitSignedLittle(binary)==v);
}

std::uint32_t Unpack24BitUnsignedBig(const std::uint8_t binary[]){
	return (static_cast<std::uint32_t>(binary[2]) ) |
		(static_cast<std::uint32_t>(binary[1]) << 8) |
		(static_cast<std::uint32_t>(binary[0]) << 16);
}

void Pack24BitUnsignedBig(std::uint8_t binary[], std::uint32_t v){
	ASSERT(binary != NULL);

	binary[2]=static_cast<std::uint8_t>(v);
	binary[1]=static_cast<std::uint8_t>(v >> 8);
	binary[0]=static_cast<std::uint8_t>(v >> 16);

	ASSERT(Unpack24BitUnsignedBig(binary)==v);
}

std::int32_t Unpack24BitSignedBig(const std::uint8_t binary[]){
	ASSERT(binary != NULL);
	std::uint32_t a = static_cast<std::uint32_t>(binary[2]) | (static_cast<std::uint32_t>(binary[1]) << 8);
	std::int32_t b = static_cast<std::int8_t>(binary[0]) << 16;
	a |= static_cast<std::uint32_t>(b);
	return static_cast<std::int32_t>(a);
}
void Pack24BitSignedBig(std::uint8_t binary[], std::int32_t v){
	ASSERT(binary != NULL);
	ASSERT(v>=kMin24BitValue && v<=kMax24BitValue);
	binary[2]=static_cast<std::uint8_t>(v);
	binary[1]=static_cast<std::uint8_t>(v >> 8);
	binary[0]=static_cast<std::uint8_t>(v >> 16);
	ASSERT(Unpack24BitSignedBig(binary)==v);
}




}	//	RSMedia

