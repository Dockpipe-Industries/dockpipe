#![cfg(target_os = "linux")]

use glib::variant::ToVariant;

#[test]
fn optimized_string_variant_iterator_preserves_values() {
    let values = ["first", "", "café", "last"];
    let variant = values.to_variant();

    assert_eq!(variant.array_iter_str().unwrap().collect::<Vec<_>>(), values);
    assert_eq!(variant.array_iter_str().unwrap().next(), Some("first"));
    assert_eq!(variant.array_iter_str().unwrap().next_back(), Some("last"));
    assert_eq!(variant.array_iter_str().unwrap().nth(2), Some("café"));
    assert_eq!(variant.array_iter_str().unwrap().nth_back(2), Some(""));
    assert_eq!(variant.array_iter_str().unwrap().last(), Some("last"));

    let mut mixed = variant.array_iter_str().unwrap();
    assert_eq!(mixed.next(), Some("first"));
    assert_eq!(mixed.next_back(), Some("last"));
    assert_eq!(mixed.next_back(), Some("café"));
    assert_eq!(mixed.next(), Some(""));
    assert_eq!(mixed.next(), None);
    assert_eq!(mixed.next_back(), None);

    let empty: [&str; 0] = [];
    let empty_variant = empty.to_variant();
    assert_eq!(empty_variant.array_iter_str().unwrap().next(), None);
}
