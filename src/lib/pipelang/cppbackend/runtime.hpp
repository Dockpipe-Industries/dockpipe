// Backend-local C++17 support. PipeLang value semantics; no Qt dependency.
#pragma once
#include <cstdint>
#include <limits>
#include <stdexcept>
#include <string>
#include <vector>
#include <utility>
namespace pipelang {
enum class Error { none, overflow, division_by_zero };
inline void valid(bool) {}
inline void valid(std::int64_t) {}
inline void valid(Error e) { if(e!=Error::overflow && e!=Error::division_by_zero) throw std::invalid_argument("invalid arithmetic error"); }
inline void valid(const std::string& s) {
  for(std::size_t i=0;i<s.size();) {
    auto c=static_cast<unsigned char>(s[i++]); if(c<0x80) continue;
    unsigned n; std::uint32_t v, minimum;
    if(c>=0xc2 && c<=0xdf){n=1;v=c&31;minimum=0x80;}
    else if(c>=0xe0 && c<=0xef){n=2;v=c&15;minimum=0x800;}
    else if(c>=0xf0 && c<=0xf4){n=3;v=c&7;minimum=0x10000;}
    else throw std::invalid_argument("invalid UTF-8");
    if(n>s.size()-i) throw std::invalid_argument("truncated UTF-8");
    while(n--){auto b=static_cast<unsigned char>(s[i++]);if((b&0xc0)!=0x80)throw std::invalid_argument("invalid UTF-8 continuation");v=(v<<6)|(b&63);}
    if(v<minimum || v>0x10ffff || (v>=0xd800 && v<=0xdfff))throw std::invalid_argument("non-scalar UTF-8");
  }
}
template<class T> struct Result { bool ok=false; T value{}; Error error=Error::overflow; };
template<class T> Result<T> success(T value){return {true,std::move(value),Error::none};}
template<class T> Result<T> failure(Error error){valid(error);return {false,T{},error};}
template<class T> bool operator==(const Result<T>& a,const Result<T>& b){return a.ok==b.ok && a.value==b.value && a.error==b.error;}
template<class T> struct Optional {bool present=false; T value{};};
template<class T> bool operator==(const Optional<T>& a,const Optional<T>& b){return a.present==b.present && a.value==b.value;}
template<class T> void valid(const std::vector<T>& xs);
template<class T> void valid(const Optional<T>& x);
template<class T> void valid(const Result<T>& x);
template<class T> void valid(const std::vector<T>& xs){for(const auto& x:xs)valid(x);}
template<class T> void valid(const Optional<T>& x){if(x.present)valid(x.value);else if(!(x.value==T{}))throw std::invalid_argument("noncanonical None");}
template<class T> void valid(const Result<T>& x){if(x.ok){if(x.error!=Error::none)throw std::invalid_argument("noncanonical Ok");valid(x.value);}else{valid(x.error);if(!(x.value==T{}))throw std::invalid_argument("noncanonical Err");}}
inline Result<std::int64_t> add(std::int64_t a,std::int64_t b){std::int64_t x;if(__builtin_add_overflow(a,b,&x))return failure<std::int64_t>(Error::overflow);return success(x);}
inline Result<std::int64_t> subtract(std::int64_t a,std::int64_t b){std::int64_t x;if(__builtin_sub_overflow(a,b,&x))return failure<std::int64_t>(Error::overflow);return success(x);}
inline Result<std::int64_t> multiply(std::int64_t a,std::int64_t b){std::int64_t x;if(__builtin_mul_overflow(a,b,&x))return failure<std::int64_t>(Error::overflow);return success(x);}
inline Result<std::int64_t> negate(std::int64_t a){return subtract(0,a);}
inline std::string add(const std::string& a,const std::string& b){return a+b;}
template<class T> std::vector<T> append(std::vector<T> xs,T x){xs.push_back(std::move(x));return xs;}
template<class T> Optional<T> at(const std::vector<T>& xs,std::int64_t i){if(i<0 || static_cast<std::uint64_t>(i)>=xs.size())return {};return {true,xs[static_cast<std::size_t>(i)]};}
// UTF-8 preserves scalar order; compare unsigned bytes explicitly, independent of char signedness.
inline int compare(const std::string& a,const std::string& b){std::size_t n=a.size()<b.size()?a.size():b.size();for(std::size_t i=0;i<n;++i){auto x=static_cast<unsigned char>(a[i]),y=static_cast<unsigned char>(b[i]);if(x!=y)return x<y?-1:1;}return a.size()==b.size()?0:(a.size()<b.size()?-1:1);}
}
