golucene
========

A [Go](http://golang.org) port of [Apache Lucene](http://lucene.apache.org).

Milestones
----------
- 2013/6/3    Initial code commited.
- 2013/11/11  First test case (gl.go) that search for specific keyword has passed with number of hits.
- 2013/12/3   Second test case (gl.go) that fetch string field has passed.
- 2014/6/29		Third test case (gl3.go) that index string field has passed.
- 2014/8/8    Migrated to Lucene 4.9.0 code base.
- 2014/8/27   Got 20 stars today. Wow~

Progress
--------
V1.0 ETA: 2014/12/1

TODOs
-----
- Used in a real blog to index posts. Working on analyzer now.
- Finish fourth test case (gl2.go) with Lucene's test framework.
- Support basic explain function.

License
-------

Until further notice, all the unreleased codes and documents are licensed under APL 2.0.

Develop Guidelines
------------------

- NewXXX method can return value or pointer. Value is for simple data object, while pointer is for more complex action object or interface when you want to protect its internal fields.
- XXXReader and XXXDirectory must be pointers as they make changes to their underlying data structure.
- Parameter in pointer form indicates mutability.
- Use interface/inheritance when children don't add new capabilities.
- Use embeded interface/struct when children add new capabilities and explicitly used.
- Prefer value internally unless (a) optional field; or (b) obvious readonly fields.
- "not implemented yet" means in scope but not yet translated from Java.
- "not supported yet" means not in scope.
- Wrapper can be implemented by composition instead of an explicit delegate field.
- Methods start with '_' are considered "internal" and vulnerable to thread-safety. They must be invoked by either other "internal" methods or synchronized methods. This is to workaround universally used re-entrant lock offered by Java synchronized keyword.

Known Risks/Issues
------------------
- Volatile keyword is not supported yet.
