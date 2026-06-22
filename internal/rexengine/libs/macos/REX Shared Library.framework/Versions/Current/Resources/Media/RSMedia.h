#pragma once

#include <cstdint>

// ### FL:
//	This should be the only Byte reader interfaces we have (no local copies in packages).
//	They should also be merged with better versions existing in AssetPackage (better, since they
//	are more distinct either streams or random access. The interfaces below are fuzzy.
//	And have no GetSize() on the reader..
//	Should also merge with CMedia/IMediaImplementation somehow.
namespace RSMedia {

typedef std::int64_t TPos;

class IByteReader {
	public: virtual void IByteReader_Read(TPos iStart, TPos iSize, std::uint8_t oBuffer[]) const=0;
};

class IByteWriter {
	public: virtual void IByteWriter_Write(TPos iStart, TPos iSize, const std::uint8_t iBuffer[])=0;

	public: virtual TPos IByteWriter_GetEnd() const=0;
};

} // RSMedia

